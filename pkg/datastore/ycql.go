package datastore

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yugabyte/gocql"

	"gofr.dev/pkg"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

// YCQL stores information about the YugabyteDB(CQL) cluster and open sessions
type YCQL struct {
	Cluster *gocql.ClusterConfig
	Session *gocql.Session
	config  CassandraCfg
	logger  log.Logger
}

//nolint:gochecknoglobals // cqlStats has to be a global variable for prometheus
var (
	cqlStats = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gofr_cql_stats",
		Help:    "Histogram for CQL",
		Buckets: []float64{.001, .003, .005, .01, .025, .05, .1, .2, .3, .4, .5, .75, 1, 2, 3, 5, 10, 30},
	}, []string{"type", "host", "keyspace"})
)

// GetNewYCQL creates and opens a connection to the NewYCQL cluster
// func GetNewYCQL(logger log.Logger, config *CassandraCfg) (YCQL, error) {
func GetNewYCQL(logger log.Logger, config *CassandraCfg) (YCQL, error) {
	const retries = 5
	// register the prometheus metric
	_ = prometheus.Register(cqlStats)

	hosts := strings.Split(config.Hosts, ",")
	cluster := gocql.NewCluster(hosts...)
	cluster.Port = config.Port
	cluster.Keyspace = config.Keyspace
	cluster.RetryPolicy = &gocql.SimpleRetryPolicy{NumRetries: retries}
	cluster.Authenticator = gocql.PasswordAuthenticator{Username: config.Username, Password: config.Password}
	cluster.Timeout = time.Duration(config.Timeout) * time.Second
	cluster.ConnectTimeout = time.Duration(config.ConnectTimeout) * time.Millisecond
	cluster.QueryObserver = logYCQL{Hosts: config.Hosts, Logger: logger, Query: make([]string, 1)}
	cluster.BatchObserver = logYCQL{Hosts: config.Hosts, Logger: logger, Query: make([]string, 1)}

	if (config.KeyFile != "" && config.CertificateFile != "") || (config.RootCertificateFile != "") {
		cluster.SslOpts = &gocql.SslOptions{CaPath: config.RootCertificateFile, KeyPath: config.KeyFile,
			CertPath: config.CertificateFile, EnableHostVerification: config.HostVerification}
		//nolint:gosec // InsecureSkipVerify can have true and false
		cluster.SslOpts.Config = &tls.Config{InsecureSkipVerify: config.InsecureSkipVerify}
	}

	if config.DataCenter != "" {
		cluster.PoolConfig.HostSelectionPolicy = gocql.DCAwareRoundRobinPolicy(config.DataCenter)
	}

	session, err := cluster.CreateSession()
	if err != nil {
		return YCQL{config: *config, logger: logger}, err
	}

	logger.Infof("Connected to YCQL with keyspace: %v", cluster.Keyspace)

	return YCQL{Cluster: cluster, Session: session, config: *config, logger: logger}, nil
}

// HealthCheck returns the health of the YCQL
func (y *YCQL) HealthCheck() types.Health {
	// handling nil object
	if y == nil {
		return types.Health{
			Name:   Ycql,
			Status: pkg.StatusDown,
		}
	}

	resp := types.Health{
		Name:     Ycql,
		Status:   pkg.StatusDown,
		Host:     y.config.Hosts,
		Database: y.config.Keyspace,
	}

	// The following check is for the condition when the connection to YCQL has not been made during initialization
	if y.Session == nil {
		y.logger.Errorf("%v", errors.HealthCheckFailed{Dependency: Ycql, Reason: "YCQL not initialized."})
		return resp
	}

	err := y.Session.Query("SELECT now() FROM system.local").Exec()
	if err != nil {
		y.logger.Error(errors.HealthCheckFailed{Dependency: Ycql, Err: err})
		return resp
	}

	resp.Status = pkg.StatusUp

	return resp
}

type logYCQL struct {
	Query  []string   `json:"query"`
	Hosts  string     `json:"host"`
	Logger log.Logger `json:"-"`
}

func (l *QueryLogger) String() string {
	line, _ := json.Marshal(l)
	return string(line)
}

// ObserveQuery monitors the query that is sent.
//
//nolint:gocritic // gocql package interface method signature cannot be changed
func (l logYCQL) ObserveQuery(_ context.Context, o gocql.ObservedQuery) {
	l.Query[0] = o.Statement
	duration := o.End.Sub(o.Start)
	ql := QueryLogger{
		Hosts:     l.Hosts,
		Query:     l.Query,
		Duration:  duration.Microseconds(),
		DataStore: Ycql,
	}

	l.Logger.Debug(ql)
	l.monitorQuery(o.Keyspace, duration.Seconds())
}

// ObserveBatch monitors the connection in a particular fixed batch
//
//nolint:gocritic // gocql package interface method signature cannot be changed
func (l logYCQL) ObserveBatch(_ context.Context, b gocql.ObservedBatch) {
	temp := strings.Join(b.Statements, ", ")
	l.Query[0] = temp
	duration := b.End.Sub(b.Start)
	ql := QueryLogger{
		Hosts:     l.Hosts,
		Query:     l.Query,
		Duration:  duration.Microseconds(),
		DataStore: Ycql,
	}

	l.Logger.Debug(ql)
	l.monitorQuery(b.Keyspace, duration.Seconds())
}

func (l logYCQL) monitorQuery(keyspace string, duration float64) {
	// push stats to prometheus
	cqlStats.WithLabelValues(checkQueryOperation(l.Query[0]), l.Hosts, keyspace).Observe(duration)
}
