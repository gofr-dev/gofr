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

type Client struct {
	config  *Config
	session *gocql.Session

	clusterConfig *gocql.ClusterConfig

	logger  Logger
	metrics Metrics
}

// New initializes Cassandra driver with the provided configuration.
// The Connect method must be called to establish a connection to Cassandra.
// Usage:
// client := New(config)
// client.UseLogger(loggerInstance)
// client.UseMetrics(metricsInstance)
// client.Connect()
func New(conf *Config) *Client {
	hosts := strings.Split(conf.Hosts, ",")
	clusterConfig := gocql.NewCluster(hosts...)
	clusterConfig.Keyspace = conf.Keyspace
	clusterConfig.Port = conf.Port
	clusterConfig.Authenticator = gocql.PasswordAuthenticator{Username: conf.Username, Password: conf.Password}

	return &Client{clusterConfig: clusterConfig}
}

// Connect establishes a connection to MongoDB and registers metrics using the provided configuration when the client was Created.
func (c *Client) Connect() {
	hosts := strings.TrimSuffix(strings.Join(c.clusterConfig.Hosts, ", "), ", ")
	c.logger.Logf("connecting to cassandra at %v on port %v to keyspace %v", c.clusterConfig.Keyspace, hosts, c.clusterConfig.Port)

	session, err := c.clusterConfig.CreateSession()
	if err != nil {
		c.logger.Error("error connecting to cassandra: ", err)

		return
	}

	cassandraBucktes := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_cassandra_stats", "Response time of CASSANDRA queries in milliseconds.", cassandraBucktes...)

	c.logger.Logf("connected to '%s' keyspace at host '%s' and port '%s'", c.clusterConfig.Keyspace, hosts, c.clusterConfig.Port)

	c.session = session
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

// Query executes the query and binds the result into dest parameter.
// Returns error if any error occurs while binding the result.
// Can be used to single as well as multiple rows.
// Accepts struct or slice of struct as dest parameter for single and multiple rows retrieval respectively
func (c *Client) Query(dest interface{}, stmt string, values ...interface{}) error {
	defer c.postProcess(&QueryLog{Query: stmt}, time.Now())

	rvo := reflect.ValueOf(dest)
	if rvo.Kind() != reflect.Ptr {
		c.logger.Error("we did not get a pointer. data is not settable.")

		return DestinationIsNotPointer{}
	}

	rv := rvo.Elem()
	iter := c.session.Query(stmt, values...).Iter()

	switch rv.Kind() {
	case reflect.Slice:
		numRows := iter.NumRows()

		for numRows > 0 {
			val := reflect.New(rv.Type().Elem())

			if rv.Type().Elem().Kind() == reflect.Struct {
				c.rowsToStruct(iter, val)
			} else {
				_ = iter.Scan(val.Interface())
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

		return UnexpectedPointer{target: rv.Kind().String()}
	}

	return nil
}

// Exec executes the query without returning any rows.
// Return error if any error occurs while executing the query
// Can be used to execute UPDATE or INSERT
func (c *Client) Exec(stmt string, values ...interface{}) error {
	defer c.postProcess(&QueryLog{Query: stmt}, time.Now())

	return c.session.Query(stmt, values...).Exec()
}

// QueryCAS executes a lightweight transaction (i.e. an UPDATE or INSERT statement containing an IF clause).
// If the transaction fails because the existing values did not match, the previous values will be stored in dest.
// Returns true if the query is applied otherwise returns false
// Returns and error if any error occur while executing the query
// Accepts only struct as the dest parameter.
func (c *Client) QueryCAS(dest interface{}, stmt string, values ...interface{}) (bool, error) {
	var (
		applied bool
		err     error
	)

	defer c.postProcess(&QueryLog{Query: stmt}, time.Now())

	rvo := reflect.ValueOf(dest)
	if rvo.Kind() != reflect.Ptr {
		c.logger.Debugf("we did not get a pointer. data is not settable.")

		return false, DestinationIsNotPointer{}
	}

	rv := rvo.Elem()
	query := c.session.Query(stmt, values...)

	switch rv.Kind() {
	case reflect.Struct:
		applied, err = c.rowsToStructCAS(query, rv)

	default:
		c.logger.Debugf("a pointer to %v was not expected.", rv.Kind().String())

		return false, UnexpectedPointer{target: rv.Kind().String()}
	}

	return applied, err
}

func (c *Client) rowsToStruct(iter *gocql.Iter, vo reflect.Value) {
	v := vo
	if vo.Kind() == reflect.Ptr {
		v = vo.Elem()
	}

	columns := c.getColumnsFromColumnsInfo(iter.Columns())
	fieldNameIndex := c.getFieldNameIndex(v)
	fields := c.getFields(columns, fieldNameIndex, v)

	_ = iter.Scan(fields...)

	if vo.CanSet() {
		vo.Set(v)
	}
}

func (c *Client) rowsToStructCAS(query *gocql.Query, vo reflect.Value) (bool, error) {
	v := vo
	if vo.Kind() == reflect.Ptr {
		v = vo.Elem()
	}

	row := make(map[string]interface{})

	applied, err := query.MapScanCAS(row)
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

func (c *Client) getColumnsFromMap(columns map[string]interface{}) []string {
	cols := make([]string, 0)

	for column := range columns {
		cols = append(cols, column)
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

	hosts := strings.TrimSuffix(strings.Join(c.clusterConfig.Hosts, ", "), ", ")

	c.metrics.RecordHistogram(context.Background(), "app_cassandra_stats", float64(duration), "hostname", hosts,
		"keyspace", c.clusterConfig.Keyspace)
}

type Health struct {
	Status  string                 `json:"status,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HealthCheck checks the health of the Cassandra.
func (c *Client) HealthCheck() interface{} {
	h := Health{
		Details: make(map[string]interface{}),
	}

	hosts := strings.TrimSuffix(strings.Join(c.clusterConfig.Hosts, ", "), ", ")

	h.Details["host"] = hosts
	h.Details["keyspace"] = c.clusterConfig.Keyspace

	if c.session == nil {
		h.Status = "DOWN"
		h.Details["message"] = "cassandra not connected"

		return &h
	}

	err := c.session.Query("SELECT now() FROM system.local").Exec()
	if err != nil {
		h.Status = "DOWN"
		h.Details["message"] = err.Error()

		return &h
	}

	h.Status = "UP"

	return &h
}
