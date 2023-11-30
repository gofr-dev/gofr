package datastore

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/prometheus/client_golang/prometheus"

	"gofr.dev/pkg"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
	"gofr.dev/pkg/middleware"
)

// MongoConfig holds the configurations for MongoDB Connectivity
type MongoConfig struct {
	HostName          string
	Port              string
	Username          string
	Password          string
	Database          string
	SSL               bool
	RetryWrites       bool
	ConnRetryDuration int
}

// MongoDB is an interface for accessing the base functionality
type MongoDB interface {
	Collection(name string, opts ...*options.CollectionOptions) *mongo.Collection
	Aggregate(ctx context.Context, pipeline interface{}, opts ...*options.AggregateOptions) (*mongo.Cursor, error)
	RunCommand(ctx context.Context, runCommand interface{}, opts ...*options.RunCmdOptions) *mongo.SingleResult
	RunCommandCursor(ctx context.Context, runCommand interface{}, opts ...*options.RunCmdOptions) (*mongo.Cursor, error)
	HealthCheck() types.Health
	IsSet() bool
}

type mongodb struct {
	*mongo.Database
	config *MongoConfig
	logger log.Logger
}

//nolint:gochecknoglobals // mongoStats has to be a global variable for prometheus
var (
	mongoStats = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    pkg.FrameworkMetricsPrefix + "mongo_stats",
		Help:    "Histogram for Mongo",
		Buckets: []float64{.001, .003, .005, .01, .025, .05, .1, .2, .3, .4, .5, .75, 1, 2, 3, 5, 10, 30},
	}, []string{"type", "host", "database"})

	_ = prometheus.Register(mongoStats)
)

func getMongoConfigFromEnv() (*MongoConfig, error) {
	getBoolEnv := func(varName string) (bool, error) {
		val := os.Getenv(varName)
		if val == "" {
			return false, nil
		}

		return strconv.ParseBool(val)
	}

	enableSSL, err := getBoolEnv("MONGO_DB_ENABLE_SSL")
	if err != nil {
		return nil, err
	}

	retryWrites, err := getBoolEnv("MONGO_DB_RETRY_WRITES")
	if err != nil {
		return nil, err
	}

	mongoConfig := MongoConfig{
		HostName:    os.Getenv("MONGO_DB_HOST"),
		Port:        os.Getenv("MONGO_DB_PORT"),
		Username:    os.Getenv("MONGO_DB_USER"),
		Password:    os.Getenv("MONGO_DB_PASS"),
		Database:    os.Getenv("MONGO_DB_NAME"),
		SSL:         enableSSL,
		RetryWrites: retryWrites,
	}

	return &mongoConfig, nil
}

// GetMongoDBFromEnv returns client to connect to MongoDB using configuration from environment variables
// Deprecated: Instead use datastore.GetNewMongoDB
func GetMongoDBFromEnv(logger log.Logger) (MongoDB, error) {
	// pushing deprecated feature count to prometheus
	middleware.PushDeprecatedFeature("GetMongoDBFromEnv")

	mongoConfig, err := getMongoConfigFromEnv()
	if err != nil {
		return mongodb{config: mongoConfig}, err
	}

	return GetNewMongoDB(logger, mongoConfig)
}

func getMongoConnectionString(config *MongoConfig) string {
	mongoConnectionString := fmt.Sprintf("mongodb://%v:%v@%v:%v/?ssl=%v&retrywrites=%v",
		config.Username,
		config.Password,
		config.HostName,
		config.Port,
		config.SSL,
		config.RetryWrites,
	)

	return mongoConnectionString
}

// GetNewMongoDB connects to MongoDB and returns the connection with the specified database in the configuration
func GetNewMongoDB(logger log.Logger, config *MongoConfig) (MongoDB, error) {
	mongoConnectionString := getMongoConnectionString(config)

	m := &mongoMonitor{database: config.Database, event: &sync.Map{},
		QueryLogger: &QueryLogger{Logger: logger, DataStore: MongoStore, Hosts: config.HostName}}
	// set client options
	clientOptions := options.Client().ApplyURI(mongoConnectionString).SetMonitor(&event.CommandMonitor{
		Started:   m.Started,
		Succeeded: m.Succeeded,
		Failed:    m.Failed,
	})

	const defaultMongoTimeout = 3
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), time.Duration(defaultMongoTimeout)*time.Second)

	defer cancel()

	// connect to MongoDB
	client, err := mongo.Connect(ctxWithTimeout, clientOptions)

	if err != nil {
		return mongodb{config: config, logger: logger}, err
	}

	// check the connection since Calling Connect does not block for server discovery. If you wish to know if a
	// MongoDB server has been found and connected to, use the Ping method
	err = client.Ping(ctxWithTimeout, nil)
	if err != nil {
		_ = client.Disconnect(ctxWithTimeout)

		return mongodb{config: config, logger: logger}, err
	}

	db := mongodb{Database: client.Database(config.Database), config: config, logger: logger}

	return db, err
}

// IsSet checks whether mongoDB is initialized or not.
func (m mongodb) IsSet() bool {
	return m.Database != nil // if connection is not nil, it will return true, if no connection, then false
}

// HealthCheck returns the health of the mongoDB
func (m mongodb) HealthCheck() types.Health {
	resp := types.Health{
		Name:     MongoStore,
		Status:   pkg.StatusDown,
		Host:     m.config.HostName,
		Database: m.config.Database,
	}
	// The following check is for the condition when the connection to MongoDB has not been made during initialization
	if m.Database == nil {
		m.logger.Errorf("%v", errors.HealthCheckFailed{Dependency: MongoStore, Reason: "MongoDB not initialized."})

		return resp
	}

	err := m.Database.RunCommand(context.Background(), bson.D{struct {
		Key   string
		Value interface{}
	}{Key: "ping", Value: 1}}, nil).Err()

	if err != nil {
		return resp
	}

	resp.Status = pkg.StatusUp

	return resp
}

// mongoMonitor will be used for logging and monitoring the query performance
type mongoMonitor struct {
	database string
	event    *sync.Map
	*QueryLogger
}

// monitorMongo logs the query at debug level and pushes the metric
func (m *mongoMonitor) monitorMongo(query, commandName string, durationNano float64) {
	const (
		nanoToMicroConv    = 1000
		microToSecondsConv = 1000000
	)

	durMicro := durationNano / nanoToMicroConv

	// push stats to prometheus
	mongoStats.WithLabelValues(commandName, m.Hosts, m.database).Observe(durMicro / microToSecondsConv)

	m.Query = []string{query}
	m.Duration = int64(durMicro)

	// log the query
	if m.Logger != nil {
		m.Logger.Debug(m.QueryLogger)
	}
}

// Started indicates that the event has started.
func (m *mongoMonitor) Started(_ context.Context, evt *event.CommandStartedEvent) {
	m.event.Store(evt.RequestID, evt.Command.Index(0).String())
}

// Succeeded indicates that the event has succeeded.
func (m *mongoMonitor) Succeeded(_ context.Context, evt *event.CommandSucceededEvent) {
	// since map gets populated for every mongo operation, we will delete the keys post operation to avoid a bulky map
	query, _ := m.event.LoadAndDelete(evt.RequestID)
	m.monitorMongo(fmt.Sprint(query), evt.CommandName, float64(evt.DurationNanos))
}

// Failed indicates that the event has failed.
func (m *mongoMonitor) Failed(_ context.Context, evt *event.CommandFailedEvent) {
	// since map gets populated for every mongo operation, we will delete the keys post operation to avoid a bulky map
	query, _ := m.event.LoadAndDelete(evt.RequestID)
	m.monitorMongo(fmt.Sprint(query), evt.CommandName, float64(evt.DurationNanos))
}
