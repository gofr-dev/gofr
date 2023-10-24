package datastore

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/extra/redisotel"
	goRedis "github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"

	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
	"gofr.dev/pkg/middleware"
)

// Redis is an abstraction that embeds the UniversalClient from go-redis/redis
type Redis interface {
	goRedis.UniversalClient
	HealthCheck() types.Health
	IsSet() bool
}

type redisClient struct {
	*goRedis.Client
	logger log.Logger
	config RedisConfig
}

type redisClusterClient struct {
	*goRedis.ClusterClient
	config RedisConfig
}

//nolint:gochecknoglobals // redisStats has to be a global variable for prometheus
var (
	redisStats = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "zs_redis_stats",
		Help:    "Histogram for Redis",
		Buckets: []float64{.001, .003, .005, .01, .025, .05, .1, .2, .3, .4, .5, .75, 1, 2, 3, 5, 10, 30},
	}, []string{"type", "host"})

	_ = prometheus.Register(redisStats)
)

// RedisConfig stores the config variables required to connect to Redis, if Options is nil, then the Redis client will import the default
// configuration as defined in go-redis/redis. User defined config can be provided by populating the Options field.
type RedisConfig struct {
	HostName                string
	Password                string
	Port                    string
	DB                      int
	SSL                     bool
	ConnectionRetryDuration int
	Options                 *goRedis.Options
}

// NewRedis connects to Redis if the given config is correct, otherwise returns the error
//
//nolint:gocognit,gocritic // cannot reduce complexity without affecting readability.
func NewRedis(logger log.Logger, config RedisConfig) (Redis, error) {
	if config.Options != nil {
		// handles the case where address might be provided through hostname and port instead of the Options.Addr
		if config.Options.Addr == "" && config.HostName != "" && config.Port != "" {
			config.Options.Addr = config.HostName + ":" + config.Port
			config.Options.DB = config.DB
		}
	} else {
		config.Options = new(goRedis.Options)
		config.Options.Addr = config.HostName + ":" + config.Port
		config.Options.DB = config.DB
	}

	config.Options.Password = config.Password

	//nolint:gosec //  using TLS 1.0
	// If SSL is enabled add TLS Config
	if config.SSL {
		config.Options.TLSConfig = &tls.Config{}
	}

	span := trace.SpanFromContext(context.Background())
	defer span.End()

	rc := goRedis.NewClient(config.Options)

	rLog := QueryLogger{
		Logger: logger,
		Hosts:  config.HostName,
	}

	rc.AddHook(&rLog)

	rc.AddHook(redisotel.TracingHook{})

	if err := rc.Ping(context.Background()).Err(); err != nil {
		// Close the redis connection
		_ = rc.Close()
		return &redisClient{logger: logger, config: config}, err
	}

	return &redisClient{logger: logger, Client: rc, config: config}, nil
}

// NewRedisFromEnv reads the config from environment variables and connects to redis if the config is correct,
// otherwise, returns the thrown error
// Deprecated: Instead use datastore.NewRedis
func NewRedisFromEnv(options *goRedis.Options) (Redis, error) {
	// pushing deprecated feature count to prometheus
	middleware.PushDeprecatedFeature("NewRedisFromEnv")

	ssl := false
	if strings.EqualFold(os.Getenv("REDIS_SSL"), "true") {
		ssl = true
	}

	config := RedisConfig{
		HostName: os.Getenv("REDIS_HOST"),
		Port:     os.Getenv("REDIS_PORT"),
		Password: os.Getenv("REDIS_PASSWORD"),
		SSL:      ssl,
	}

	if options != nil {
		config.Options = options
	}

	return NewRedis(log.NewLogger(), config)
}

// NewRedisCluster returns a new Redis cluster client object if the given config is correct, otherwise returns the error
func NewRedisCluster(clusterOptions *goRedis.ClusterOptions) (Redis, error) {
	rcc := goRedis.NewClusterClient(clusterOptions)

	if err := rcc.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return &redisClusterClient{ClusterClient: rcc, config: RedisConfig{HostName: strings.Join(clusterOptions.Addrs, ",")}}, nil
}

// HealthCheck returns the health of the redis DB
func (r *redisClient) HealthCheck() types.Health {
	resp := types.Health{
		Name:   RedisStore,
		Status: pkg.StatusDown,
		Host:   r.config.HostName,
	}

	// The following check is for the connection when the connection to Redis has not been made during initialization
	if r.Client == nil {
		r.logger.Errorf("%v", errors.HealthCheckFailed{Dependency: "Redis", Reason: "Redis not initialized"})
		return resp
	}

	err := r.Client.Ping(context.Background()).Err()
	if err != nil {
		r.logger.Errorf("%v", errors.HealthCheckFailed{Dependency: "Redis", Err: err})
		return resp
	}

	resp.Status = pkg.StatusUp
	resp.Details = r.getInfoInMap()

	return resp
}

// getInfoInMap runs the INFO command on redis and returns a structured map divided by sections of redis response.
func (r *redisClient) getInfoInMap() map[string]map[string]string {
	info, _ := r.Client.Info(context.Background(), "all").Result()

	result := make(map[string]map[string]string)
	parts := strings.Split(info, "\r\n")

	var currentSection string

	for _, p := range parts {
		// Take care of empty lines
		if p == "" {
			continue
		}

		// Take care of section headers
		if strings.HasPrefix(p, "#") {
			currentSection = strings.ToLower(strings.TrimPrefix(p, "# "))
			result[currentSection] = make(map[string]string)

			continue
		}

		// Normal lines
		splits := strings.Split(p, ":")
		result[currentSection][splits[0]] = splits[1]
	}

	return result
}

// HealthCheck returns the health of the redis DB cluster
func (r *redisClusterClient) HealthCheck() types.Health {
	resp := types.Health{
		Name:   RedisStore,
		Status: pkg.StatusDown,
		Host:   r.config.HostName,
	}

	// The following check is for the connection when the connection to Redis has not been made during initialization
	if r.ClusterClient == nil {
		return resp
	}

	err := r.ClusterClient.Ping(context.Background()).Err()
	if err != nil {
		return resp
	}

	resp.Status = pkg.StatusUp

	return resp
}

// IsSet checks whether redis is initialized or not
func (r *redisClient) IsSet() bool {
	return r.Client != nil // will return true when client is set
}

// IsSet checks whether redis cluster is initialized or not
func (r *redisClusterClient) IsSet() bool {
	return r.ClusterClient != nil // will return true when client is set
}

// BeforeProcess sets the start time in the context and returns it.
func (l *QueryLogger) BeforeProcess(ctx context.Context, _ goRedis.Cmder) (context.Context, error) {
	l.StartTime = time.Now()

	return ctx, nil
}

// AfterProcess sets the metrics such as endTime, duration after the completion of the process
func (l *QueryLogger) AfterProcess(_ context.Context, cmd goRedis.Cmder) error {
	endTime := time.Now()
	query := fmt.Sprintf("%v", cmd.Args())
	query = strings.TrimPrefix(query, "[")
	query = strings.TrimSuffix(query, "]")
	l.Duration = endTime.Sub(l.StartTime).Microseconds()
	l.Query = make([]string, 1)
	l.Query[0] = query
	s := strings.Split(l.Query[0], " ")
	l.DataStore = RedisStore
	dur := endTime.Sub(l.StartTime).Seconds()

	l.monitorRedis(s, dur)

	return nil
}

// BeforeProcessPipeline sets the start time for slice of redis.Cmder in the context and returns it.
func (l *QueryLogger) BeforeProcessPipeline(ctx context.Context, _ []goRedis.Cmder) (context.Context, error) {
	l.StartTime = time.Now()

	return ctx, nil
}

// AfterProcessPipeline sets the metrics such as endTime, duration after the completion of the process
func (l *QueryLogger) AfterProcessPipeline(_ context.Context, cmds []goRedis.Cmder) error {
	l.Query = make([]string, len(cmds))
	endTime := time.Now()

	for i := range cmds {
		query := fmt.Sprintf("%v", cmds[i].Args())
		query = strings.TrimPrefix(query, "[")
		query = strings.TrimSuffix(query, "]")
		l.Query[i] = query
	}

	query := strings.Split(l.Query[0], " ")

	l.Duration = endTime.Sub(l.StartTime).Microseconds()
	l.DataStore = RedisStore

	dur := endTime.Sub(l.StartTime).Seconds()

	l.monitorRedis(query, dur)

	return nil
}

func (l *QueryLogger) monitorRedis(query []string, duration float64) {
	l.Logger.Debug(l)
	// push stats to prometheus
	redisStats.WithLabelValues(query[0], l.Hosts).Observe(duration)
}
