package datasource

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/logger"
)

type contextKey string

const redisStartTimeKey, redisPipelineStartTime contextKey = "redisStartTime", "redisPipelineStartTime"

type RedisConfig struct {
	HostName string
	Port     int
	Options  *redis.Options
}

type queryLogger struct {
	Query     []string      `json:"query"`
	Duration  int64         `json:"duration"`
	StartTime time.Time     `json:"-"`
	Logger    logger.Logger `json:"-"`
}

// newRedisClient return a redis client if connection is successful based on Config.
// In case of error, it returns an error as second parameter.
func NewRedisClient(config RedisConfig) (*redis.Client, error) {
	if config.Options == nil {
		config.Options = new(redis.Options)
	}

	if config.Options.Addr == "" && config.HostName != "" && config.Port != 0 {
		config.Options.Addr = fmt.Sprintf("%s:%d", config.HostName, config.Port)
	}

	rc := redis.NewClient(config.Options)
	if err := rc.Ping(context.TODO()).Err(); err != nil {
		return nil, err
	}

	if err := redisotel.InstrumentTracing(rc); err != nil {
		panic(err)
	}

	// Enable metrics instrumentation.
	// if err := redisotel.InstrumentMetrics(rc); err != nil {
	//	panic(err)
	// }

	return rc, nil
}

// TODO - if we make Redis an interface and expose from container we can avoid c.Redis(c, command) using methods on c and still pass c.
// type Redis interface {
//	Get(string) (string, error)
// }

// BeforeRedisCommand method is called before a single Redis command is executed useful for recording the start time of the operation.
func (l *queryLogger) BeforeRedisCommand(ctx context.Context) context.Context {
	return context.WithValue(ctx, redisStartTimeKey, time.Now())
}

// AfterRedisCommand method is called after a single Redis command is executed. Common use cases include logging the
// command details, measuring the duration of the command execution, and handling any errors that may have occurred.
func (l *queryLogger) AfterRedisCommand(ctx context.Context, cmd redis.Cmder) error {
	startTime, ok := ctx.Value(redisStartTimeKey).(time.Time)
	if !ok {
		l.Logger.Error("Failed to retrieve start time from context")
		return nil
	}

	endTime := time.Now()
	query := formatRedisQuery(cmd.Args()...)
	l.Duration = endTime.Sub(l.StartTime).Microseconds()
	l.Query = []string{query}
	s := strings.Split(query, " ")

	duration := endTime.Sub(startTime).Microseconds()

	l.Logger.Debugf("Redis query: %s, duration: %dµs", s[0], duration)

	return nil
}

func (l *queryLogger) BeforeProcessPipeline(ctx context.Context) (context.Context, error) {
	return context.WithValue(ctx, redisPipelineStartTime, time.Now()), nil
}

func (l *queryLogger) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	startTime, ok := ctx.Value(redisPipelineStartTime).(time.Time)
	if !ok {
		l.Logger.Error("Failed to retrieve pipeline start time from context")
		return nil
	}

	endTime := time.Now()

	// Format queries as a single string with newlines for readability
	queries := make([]string, len(cmds))

	for i, cmd := range cmds {
		query := formatRedisQuery(cmd.Args()...)
		queries[i] = query
	}

	query := strings.Join(queries, " , ")

	duration := endTime.Sub(startTime).Microseconds()

	l.Logger.Debugf("Redis pipeline: %d queries : [ %v ] ,duration: %dµs", len(cmds), query, duration)

	return nil
}

func formatRedisQuery(args ...interface{}) string {
	formattedArgs := make([]string, 0)
	for _, arg := range args {
		formattedArgs = append(formattedArgs, fmt.Sprintf("%v", arg))
	}

	return strings.Join(formattedArgs, " ")
}
