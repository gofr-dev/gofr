package datastore

import (
	"context"
	"crypto/tls"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"github.com/prometheus/client_golang/prometheus"

	"gofr.dev/pkg"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

const (
	localQuorum = "LOCAL_QUORUM"
)

// CassandraCfg holds the configurations for Cassandra Connectivity
type CassandraCfg struct {
	Hosts               string
	Consistency         string
	Username            string
	Password            string
	Keyspace            string
	RetryPolicy         gocql.RetryPolicy
	CertificateFile     string
	KeyFile             string
	RootCertificateFile string
	DataCenter          string
	Port                int
	Timeout             int
	ConnectTimeout      int
	ConnRetryDuration   int
	TLSVersion          uint16
	HostVerification    bool
	InsecureSkipVerify  bool
}

// Cassandra stores information about the Cassandra cluster and open sessions
type Cassandra struct {
	Cluster *gocql.ClusterConfig
	Session *gocql.Session
	config  CassandraCfg
	logger  log.Logger
}

// GetNewCassandra creates and opens a connection to the cassandra cluster
func GetNewCassandra(logger log.Logger, cassandraCfg *CassandraCfg) (Cassandra, error) {
	const maxRetries = 10

	const interval = 8
	// register the prometheus metric
	_ = prometheus.Register(cqlStats)

	hosts := strings.Split(cassandraCfg.Hosts, ",")
	cluster := gocql.NewCluster(hosts...)
	cluster.Port = cassandraCfg.Port
	cluster.Timeout = time.Duration(cassandraCfg.Timeout) * time.Second
	cluster.ConnectTimeout = time.Duration(cassandraCfg.ConnectTimeout) * time.Millisecond
	cluster.ReconnectionPolicy = &gocql.ConstantReconnectionPolicy{MaxRetries: maxRetries, Interval: interval * time.Second}
	cluster.Keyspace = cassandraCfg.Keyspace
	cluster.Authenticator = gocql.PasswordAuthenticator{Username: cassandraCfg.Username, Password: cassandraCfg.Password}
	cluster.RetryPolicy = cassandraCfg.RetryPolicy
	cluster.QueryObserver = QueryLogger{Hosts: cassandraCfg.Hosts, Logger: logger, Query: make([]string, 1)}
	cluster.BatchObserver = QueryLogger{Hosts: cassandraCfg.Hosts, Logger: logger, Query: make([]string, 1)}

	if cassandraCfg.RootCertificateFile != "" {
		cluster.SslOpts = &gocql.SslOptions{CaPath: cassandraCfg.RootCertificateFile, KeyPath: cassandraCfg.KeyFile,
			CertPath: cassandraCfg.CertificateFile, EnableHostVerification: cassandraCfg.HostVerification}
		//nolint:gosec // InsecureSkipVerify can have true and false
		cluster.SslOpts.Config = &tls.Config{InsecureSkipVerify: cassandraCfg.InsecureSkipVerify}
	}

	if cassandraCfg.DataCenter != "" {
		cluster.PoolConfig.HostSelectionPolicy = gocql.DCAwareRoundRobinPolicy(cassandraCfg.DataCenter)
	}

	session, err := cluster.CreateSession()
	if err != nil {
		return Cassandra{config: *cassandraCfg, logger: logger}, err
	}

	logger.Infof("Connected to cassandra with keyspace: %v", cluster.Keyspace)

	return Cassandra{Cluster: cluster, Session: session, config: *cassandraCfg, logger: logger}, nil
}

func enableHostVerification(enableVerification string) bool {
	return enableVerification != "false"
}

func setTLSVersion(version string) uint16 {
	if version == "10" {
		return tls.VersionTLS10
	} else if version == "11" {
		return tls.VersionTLS11
	} else if version == "13" {
		return tls.VersionTLS13
	}

	return tls.VersionTLS12
}

// HealthCheck returns the health of Cassandra
func (c *Cassandra) HealthCheck() types.Health {
	// handling nil object
	if c == nil {
		return types.Health{
			Name:   CassandraStore,
			Status: pkg.StatusDown,
		}
	}

	resp := types.Health{
		Name:     CassandraStore,
		Status:   pkg.StatusDown,
		Host:     c.config.Hosts,
		Database: c.config.Keyspace,
	}

	// The following check is for the condition when the connection to Cassandra has not been made during initialization
	if c.Session == nil {
		c.logger.Errorf("%v", errors.HealthCheckFailed{Dependency: CassandraStore, Reason: "Cassandra not initialized."})
		return resp
	}

	err := c.Session.Query("SELECT now() FROM system.local").Exec()
	if err != nil {
		c.logger.Errorf("%v", errors.HealthCheckFailed{Dependency: CassandraStore, Err: err})
		return resp
	}

	resp.Status = pkg.StatusUp

	return resp
}

// ObserveQuery monitors the query that is sent.
//
//nolint:gocritic // gocql package interface method signature cannot be changed
func (l QueryLogger) ObserveQuery(_ context.Context, o gocql.ObservedQuery) {
	duration := o.End.Sub(o.Start)
	l.Query[0] = o.Statement
	l.Duration = duration.Microseconds()
	l.DataStore = CassandraStore

	l.Logger.Debug(l)
	l.monitorQuery(o.Keyspace, duration.Seconds())
}

// ObserveBatch monitors the connection in a particular fixed batch
//
//nolint:gocritic // gocql package interface method signature cannot be changed
func (l QueryLogger) ObserveBatch(_ context.Context, b gocql.ObservedBatch) {
	duration := b.End.Sub(b.Start)
	temp := strings.Join(b.Statements, ", ")
	l.Query[0] = temp
	l.Duration = duration.Microseconds()
	l.DataStore = CassandraStore

	l.Logger.Debug(l)
	l.monitorQuery(b.Keyspace, duration.Seconds())
}

func (l *QueryLogger) monitorQuery(keyspace string, duration float64) {
	// push stats to prometheus
	cqlStats.WithLabelValues(checkQueryOperation(l.Query[0]), l.Hosts, keyspace).Observe(duration)
}
