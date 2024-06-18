package cassandra

import (
	"context"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gocql/gocql"
)

type Config struct {
	Hosts    string
	Keyspace string
	Port     int
	Username string
	Password string
}

type cassandra struct {
	clusterConfig clusterConfig
	session       session
	query         query
}

type Client struct {
	config *Config

	cassandra *cassandra

	logger  Logger
	metrics Metrics
}

// New initializes Cassandra driver with the provided configuration.
// The Connect method must be called to establish a connection to Cassandra.
// Usage:
//
//	client := New(config)
//	client.UseLogger(loggerInstance)
//	client.UseMetrics(metricsInstance)
//	client.Connect()
func New(conf Config) *Client {
	cass := &cassandra{clusterConfig: newClusterConfig(&conf)}

	return &Client{config: &conf, cassandra: cass}
}

// Connect establishes a connection to Cassandra and registers metrics using the provided configuration when the client was Created.
func (c *Client) Connect() {
	c.logger.Logf("connecting to cassandra at %v on port %v to keyspace %v", c.config.Hosts, c.config.Port, c.config.Keyspace)

	sess, err := c.cassandra.clusterConfig.createSession()
	if err != nil {
		c.logger.Error("error connecting to cassandra: ", err)

		return
	}

	cassandraBucktes := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_cassandra_stats", "Response time of CASSANDRA queries in milliseconds.", cassandraBucktes...)

	c.logger.Logf("connected to '%s' keyspace at host '%s' and port '%d'", c.config.Keyspace, c.config.Hosts, c.config.Port)

	c.cassandra.session = sess
}

// UseLogger sets the logger for the Cassandra client which asserts the Logger interface.
func (c *Client) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the Cassandra client which asserts the Metrics interface.
func (c *Client) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

//nolint:exhaustive // We just want to take care of slice and struct in this case.
func (c *Client) Query(dest interface{}, stmt string, values ...interface{}) error {
	defer c.postProcess(&QueryLog{Query: stmt, Keyspace: c.config.Keyspace}, time.Now())

	rvo := reflect.ValueOf(dest)
	if rvo.Kind() != reflect.Ptr {
		c.logger.Error("we did not get a pointer. data is not settable.")

		return destinationIsNotPointer{}
	}

	rv := rvo.Elem()
	iter := c.cassandra.session.query(stmt, values...).iter()

	switch rv.Kind() {
	case reflect.Slice:
		numRows := iter.numRows()

		for numRows > 0 {
			val := reflect.New(rv.Type().Elem())

			if rv.Type().Elem().Kind() == reflect.Struct {
				c.rowsToStruct(iter, val)
			} else {
				_ = iter.scan(val.Interface())
			}

			rv = reflect.Append(rv, val.Elem())

			numRows--
		}

		if rvo.Elem().CanSet() {
			rvo.Elem().Set(rv)
		}

	case reflect.Struct:
		c.rowsToStruct(iter, rv)

	default:
		c.logger.Debugf("a pointer to %v was not expected.", rv.Kind().String())

		return unexpectedPointer{target: rv.Kind().String()}
	}

	return nil
}

func (c *Client) Exec(stmt string, values ...interface{}) error {
	defer c.postProcess(&QueryLog{Query: stmt, Keyspace: c.config.Keyspace}, time.Now())

	return c.cassandra.session.query(stmt, values...).exec()
}

//nolint:exhaustive // We just want to take care of slice and struct in this case.
func (c *Client) ExecCAS(dest interface{}, stmt string, values ...interface{}) (bool, error) {
	var (
		applied bool
		err     error
	)

	defer c.postProcess(&QueryLog{Query: stmt, Keyspace: c.config.Keyspace}, time.Now())

	rvo := reflect.ValueOf(dest)
	if rvo.Kind() != reflect.Ptr {
		c.logger.Debugf("we did not get a pointer. data is not settable.")

		return false, destinationIsNotPointer{}
	}

	rv := rvo.Elem()
	q := c.cassandra.session.query(stmt, values...)

	switch rv.Kind() {
	case reflect.Struct:
		applied, err = c.rowsToStructCAS(q, rv)

	case reflect.Slice:
		c.logger.Debugf("a slice of %v was not expected.", reflect.SliceOf(reflect.TypeOf(dest)).String())

		return false, unexpectedSlice{target: reflect.SliceOf(reflect.TypeOf(dest)).String()}

	case reflect.Map:
		c.logger.Debugf("a map was not expected.")

		return false, unexpectedMap{}

	default:
		applied, err = q.scanCAS(rv.Interface())
	}

	return applied, err
}

func (c *Client) rowsToStruct(iter iterator, vo reflect.Value) {
	v := vo
	if vo.Kind() == reflect.Ptr {
		v = vo.Elem()
	}

	columns := c.getColumnsFromColumnsInfo(iter.columns())
	fieldNameIndex := c.getFieldNameIndex(v)
	fields := c.getFields(columns, fieldNameIndex, v)

	_ = iter.scan(fields...)

	if vo.CanSet() {
		vo.Set(v)
	}
}

func (c *Client) rowsToStructCAS(query query, vo reflect.Value) (bool, error) {
	v := vo
	if vo.Kind() == reflect.Ptr {
		v = vo.Elem()
	}

	row := make(map[string]interface{})

	applied, err := query.mapScanCAS(row)
	if err != nil {
		return false, err
	}

	fieldNameIndex := c.getFieldNameIndex(v)

	for col, value := range row {
		if i, ok := fieldNameIndex[col]; ok {
			field := v.Field(i)
			if reflect.TypeOf(value) == field.Type() {
				field.Set(reflect.ValueOf(value))
			}
		}
	}

	if vo.CanSet() {
		vo.Set(v)
	}

	return applied, nil
}

func (c *Client) getFields(columns []string, fieldNameIndex map[string]int, v reflect.Value) []interface{} {
	fields := make([]interface{}, 0)

	for _, column := range columns {
		if i, ok := fieldNameIndex[column]; ok {
			fields = append(fields, v.Field(i).Addr().Interface())
		} else {
			var i interface{}
			fields = append(fields, &i)
		}
	}

	return fields
}

func (c *Client) getFieldNameIndex(v reflect.Value) map[string]int {
	fieldNameIndex := map[string]int{}

	for i := 0; i < v.Type().NumField(); i++ {
		var name string

		f := v.Type().Field(i)
		tag := f.Tag.Get("db")

		if tag != "" {
			name = tag
		} else {
			name = toSnakeCase(f.Name)
		}

		fieldNameIndex[name] = i
	}

	return fieldNameIndex
}

func (c *Client) getColumnsFromColumnsInfo(columns []gocql.ColumnInfo) []string {
	cols := make([]string, 0)

	for _, column := range columns {
		cols = append(cols, column.Name)
	}

	return cols
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func toSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")

	return strings.ToLower(snake)
}

func (c *Client) postProcess(ql *QueryLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	ql.Duration = duration

	c.logger.Debug(ql)

	c.metrics.RecordHistogram(context.Background(), "app_cassandra_stats", float64(duration), "hostname", c.config.Hosts,
		"keyspace", c.config.Keyspace)

	c.cassandra.query = nil
}

type Health struct {
	Status  string                 `json:"status,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HealthCheck checks the health of the Cassandra.
func (c *Client) HealthCheck() interface{} {
	const (
		statusDown = "DOWN"
		statusUp   = "UP"
	)

	h := Health{
		Details: make(map[string]interface{}),
	}

	h.Details["host"] = c.config.Hosts
	h.Details["keyspace"] = c.config.Keyspace

	if c.cassandra.session == nil {
		h.Status = statusDown
		h.Details["message"] = "cassandra not connected"

		return &h
	}

	err := c.cassandra.session.query("SELECT now() FROM system.local").exec()
	if err != nil {
		h.Status = statusDown
		h.Details["message"] = err.Error()

		return &h
	}

	h.Status = statusUp

	return &h
}

// cassandraIterator implements iterator interface.
type cassandraIterator struct {
	iter *gocql.Iter
}

// Columns gets the column information.
// This method wraps the `Columns` method of the underlying `iter` object.
func (c *cassandraIterator) columns() []gocql.ColumnInfo {
	return c.iter.Columns()
}

// Scan gets the next row from the Cassandra iterator and fills in the provided arguments.
// This method wraps the `Scan` method of the underlying `iter` object.
func (c *cassandraIterator) scan(dest ...interface{}) bool {
	return c.iter.Scan(dest...)
}

// NumRows returns a number of rows.
// This method wraps the `NumRows` method of the underlying `iter` object.
func (c *cassandraIterator) numRows() int {
	return c.iter.NumRows()
}

// cassandraQuery implements query interface.
type cassandraQuery struct {
	query *gocql.Query
}

// Exec performs a Cassandra's Query Exec.
// This method wraps the `Exec` method of the underlying `query` object.
func (c *cassandraQuery) exec() error {
	return c.query.Exec()
}

// Iter returns a Cassandra iterator.
// This method wraps the `Iter` method of the underlying `query` object.
func (c *cassandraQuery) iter() iterator {
	iter := cassandraIterator{iter: c.query.Iter()}

	return &iter
}

// MapScanCAS checks a Cassandra query with an IF clause and scans the existing data into map[string]interface{} (if any).
// This method wraps the `MapScanCAS` method of the underlying `query` object.
func (c *cassandraQuery) mapScanCAS(dest map[string]interface{}) (applied bool, err error) {
	return c.query.MapScanCAS(dest)
}

// ScanCAS checks a Cassandra query with an IF clause and scans the existing data (if any).
// This method wraps the `ScanCAS` method of the underlying `query` object.
func (c *cassandraQuery) scanCAS(dest ...any) (applied bool, err error) {
	return c.query.ScanCAS(dest)
}

// cassandraClusterConfig implements clusterConfig interface.
type cassandraClusterConfig struct {
	clusterConfig *gocql.ClusterConfig
}

func newClusterConfig(config *Config) clusterConfig {
	var c cassandraClusterConfig

	config.Hosts = strings.TrimSuffix(strings.TrimSpace(config.Hosts), ",")
	hosts := strings.Split(config.Hosts, ",")
	c.clusterConfig = gocql.NewCluster(hosts...)
	c.clusterConfig.Keyspace = config.Keyspace
	c.clusterConfig.Port = config.Port
	c.clusterConfig.Authenticator = gocql.PasswordAuthenticator{Username: config.Username, Password: config.Password}

	return &c
}

// CreateSession creates a Cassandra session based on the provided configuration.
// This method wraps the `CreateSession` method of the underlying `clusterConfig` object.
// It creates a new Cassandra session using the configuration options specified in `c.clusterConfig`.
//
// Returns:
//   - A `session` object representing the established Cassandra connection, or `nil` if an error occurred.
//   - An `error` object if there was a problem creating the session, or `nil` if successful.
func (c *cassandraClusterConfig) createSession() (session, error) {
	sess, err := c.clusterConfig.CreateSession()
	if err != nil {
		return nil, err
	}

	return &cassandraSession{session: sess}, nil
}

// cassandraSession implements session interface.
type cassandraSession struct {
	session *gocql.Session
}

// Query creates a Cassandra query.
// This method wraps the `Query` method of the underlying `session` object.
func (c *cassandraSession) query(stmt string, values ...interface{}) query {
	q := &cassandraQuery{query: c.session.Query(stmt, values...)}

	return q
}
