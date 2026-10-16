package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/mock/gomock"
)

func getClickHouseTestConnection(t *testing.T) (*MockConn, *MockMetrics, *MockLogger, Client) {
	t.Helper()

	ctrl := gomock.NewController(t)

	mockConn := NewMockConn(ctrl)
	mockMetric := NewMockMetrics(ctrl)
	mockLogger := NewMockLogger(ctrl)

	c := Client{conn: mockConn, config: Config{
		Hosts:    "localhost",
		Username: "user",
		Password: "pass",
		Database: "test",
	}, logger: mockLogger, metrics: mockMetric}

	return mockConn, mockMetric, mockLogger, c
}

func Test_ClickHouse_Options_PoolConfigPassedThrough(t *testing.T) {
	opts := clickHouseOptions(&Config{
		Hosts:           "host-a:9000,host-b:9000",
		Username:        "user",
		Password:        "pass",
		Database:        "test",
		MaxOpenConns:    30,
		MaxIdleConns:    10,
		DialTimeout:     7 * time.Second,
		ConnMaxLifetime: 15 * time.Minute,
	})

	assert.Equal(t, []string{"host-a:9000", "host-b:9000"}, opts.Addr)
	assert.Equal(t, "test", opts.Auth.Database)
	assert.Equal(t, "user", opts.Auth.Username)
	assert.Equal(t, "pass", opts.Auth.Password)
	assert.Equal(t, 30, opts.MaxOpenConns)
	assert.Equal(t, 10, opts.MaxIdleConns)
	assert.Equal(t, 7*time.Second, opts.DialTimeout)
	assert.Equal(t, 15*time.Minute, opts.ConnMaxLifetime)
}

func Test_ClickHouse_Options_HostsTrimmedAndEmptiesDropped(t *testing.T) {
	tests := []struct {
		name  string
		hosts string
		want  []string
	}{
		{"spaces after comma", "host-a:9000, host-b:9000", []string{"host-a:9000", "host-b:9000"}},
		{"trailing comma", "host-a:9000,", []string{"host-a:9000"}},
		{"surrounding and interior blanks", " host-a:9000 , , host-b:9000 ", []string{"host-a:9000", "host-b:9000"}},
		{"single host", "localhost:9000", []string{"localhost:9000"}},
		{"empty string", "", []string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := clickHouseOptions(&Config{Hosts: tc.hosts, Database: "test"})
			assert.Equal(t, tc.want, opts.Addr)
		})
	}
}

func Test_ClickHouse_Options_ZeroPoolConfigLeavesDefaultsToDriver(t *testing.T) {
	// Unset pool fields must pass through as zero so clickhouse-go applies its
	// own defaults — i.e. behavior is identical to before this option existed.
	opts := clickHouseOptions(&Config{Hosts: "localhost:9000", Database: "test"})

	assert.Equal(t, []string{"localhost:9000"}, opts.Addr)
	assert.Zero(t, opts.MaxOpenConns)
	assert.Zero(t, opts.MaxIdleConns)
	assert.Zero(t, opts.DialTimeout)
	assert.Zero(t, opts.ConnMaxLifetime)
}

func Test_ClickHouse_ConnectAndMetricRegistrationAndPingFailure(t *testing.T) {
	_, mockMetric, mockLogger, _ := getClickHouseTestConnection(t)

	cl := New(Config{
		Hosts:    "localhost:8000",
		Username: "user",
		Password: "pass",
		Database: "test",
	})

	cl.UseLogger(mockLogger)
	cl.UseMetrics(mockMetric)

	mockMetric.EXPECT().NewHistogram("app_clickhouse_stats", "Response time of Clickhouse queries in microseconds.", gomock.Any())
	mockMetric.EXPECT().NewGauge("app_clickhouse_open_connections", "Number of open Clickhouse connections.")
	mockMetric.EXPECT().NewGauge("app_clickhouse_idle_connections", "Number of idle Clickhouse connections.")
	mockMetric.EXPECT().SetGauge("app_clickhouse_open_connections", gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge("app_clickhouse_idle_connections", gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debugf("connecting to Clickhouse db at %v to database %v", "localhost:8000", "test")
	mockLogger.EXPECT().Errorf("ping failed with error %v", gomock.Any())

	cl.Connect()

	time.Sleep(100 * time.Millisecond)

	assert.True(t, mockLogger.ctrl.Satisfied())
	assert.True(t, mockMetric.ctrl.Satisfied())
}

func Test_ClickHouse_HealthUP(t *testing.T) {
	mockConn, _, _, c := getClickHouseTestConnection(t)

	mockConn.EXPECT().Ping(gomock.Any()).Return(nil)

	resp, _ := c.HealthCheck(context.Background())

	assert.Contains(t, fmt.Sprint(resp), "UP")
}

func Test_ClickHouse_HealthDOWN(t *testing.T) {
	mockConn, _, _, c := getClickHouseTestConnection(t)

	mockConn.EXPECT().Ping(gomock.Any()).Return(sql.ErrConnDone)

	resp, err := c.HealthCheck(context.Background())

	require.ErrorIs(t, err, errStatusDown)

	assert.Contains(t, fmt.Sprint(resp), "DOWN")
}

func Test_ClickHouse_Exec(t *testing.T) {
	mockConn, mockMetric, mockLogger, c := getClickHouseTestConnection(t)

	ctx := context.Background()

	mockConn.EXPECT().Exec(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)",
		"8f165e2d-feef-416c-95f6-913ce3172e15", "gofr", "10").Return(nil)

	mockLogger.EXPECT().Debug(gomock.Any())

	mockMetric.EXPECT().RecordHistogram(ctx, "app_clickhouse_stats", gomock.Any(), "hosts", c.config.Hosts,
		"database", c.config.Database, "type", "INSERT")

	err := c.Exec(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)", "8f165e2d-feef-416c-95f6-913ce3172e15", "gofr", "10")

	require.NoError(t, err)
}

func Test_ClickHouse_Select(t *testing.T) {
	mockConn, mockMetric, mockLogger, c := getClickHouseTestConnection(t)

	type User struct {
		ID   string `ch:"id"`
		Name string `ch:"name"`
		Age  string `ch:"age"`
	}

	ctx := context.Background()

	var user []User

	mockConn.EXPECT().Select(ctx, &user, "SELECT * FROM users").Return(nil)

	mockLogger.EXPECT().Debug(gomock.Any())

	mockMetric.EXPECT().RecordHistogram(ctx, "app_clickhouse_stats", gomock.Any(), "hosts", c.config.Hosts,
		"database", c.config.Database, "type", "SELECT")

	err := c.Select(ctx, &user, "SELECT * FROM users")

	require.NoError(t, err)
}

func Test_ClickHouse_AsyncInsert(t *testing.T) {
	mockConn, mockMetric, mockLogger, c := getClickHouseTestConnection(t)

	ctx := context.Background()

	mockConn.EXPECT().AsyncInsert(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)", true,
		"8f165e2d-feef-416c-95f6-913ce3172e15", "user", "10").Return(nil)

	mockMetric.EXPECT().RecordHistogram(ctx, "app_clickhouse_stats", gomock.Any(), "hosts", c.config.Hosts,
		"database", c.config.Database, "type", "INSERT")

	mockLogger.EXPECT().Debug(gomock.Any())

	err := c.AsyncInsert(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)", true,
		"8f165e2d-feef-416c-95f6-913ce3172e15", "user", "10")

	require.NoError(t, err)
}

// slowCall is how long the fake driver takes; every recorded duration must cover it.
const slowCall = 50 * time.Millisecond

var errDriver = errors.New("driver failed")

// recordingSpan keeps the attributes set on it and counts End calls.
type recordingSpan struct {
	noop.Span

	mu    sync.Mutex
	attrs map[attribute.Key]attribute.Value
	ended int
}

func (s *recordingSpan) SetAttributes(kv ...attribute.KeyValue) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, a := range kv {
		s.attrs[a.Key] = a.Value
	}
}

func (s *recordingSpan) End(...trace.SpanEndOption) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ended++
}

// recordingTracer hands out a single recordingSpan.
type recordingTracer struct {
	noop.Tracer

	span *recordingSpan
}

func (t *recordingTracer) Start(ctx context.Context, _ string, _ ...trace.SpanStartOption) (context.Context, trace.Span) {
	return ctx, t.span
}

func Test_ClickHouse_OperationDurationCoversTheDriverCall(t *testing.T) {
	const query = "SELECT id FROM users WHERE id = ?"

	var dest []struct{}

	tests := []struct {
		desc      string
		span      string
		driverErr error
		expect    func(conn *MockConn, driverErr error)
		call      func(ctx context.Context, c *Client) error
	}{
		{desc: "exec", span: "exec", expect: expectSlowExec, call: func(ctx context.Context, c *Client) error {
			return c.Exec(ctx, query, 1)
		}},
		{desc: "exec error", span: "exec", driverErr: errDriver, expect: expectSlowExec, call: func(ctx context.Context, c *Client) error {
			return c.Exec(ctx, query, 1)
		}},
		{desc: "select", span: "select", expect: expectSlowSelect(&dest), call: func(ctx context.Context, c *Client) error {
			return c.Select(ctx, &dest, query, 1)
		}},
		{desc: "select error", span: "select", driverErr: errDriver, expect: expectSlowSelect(&dest),
			call: func(ctx context.Context, c *Client) error {
				return c.Select(ctx, &dest, query, 1)
			}},
		{desc: "async insert", span: "async-insert", expect: expectSlowAsyncInsert, call: func(ctx context.Context, c *Client) error {
			return c.AsyncInsert(ctx, query, true, 1)
		}},
		{desc: "async insert error", span: "async-insert", driverErr: errDriver, expect: expectSlowAsyncInsert,
			call: func(ctx context.Context, c *Client) error {
				return c.AsyncInsert(ctx, query, true, 1)
			}},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			mockConn, mockMetric, mockLogger, c := getClickHouseTestConnection(t)

			span := &recordingSpan{attrs: map[attribute.Key]attribute.Value{}}
			c.tracer = &recordingTracer{span: span}

			tc.expect(mockConn, tc.driverErr)

			var observed float64

			mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_clickhouse_stats", gomock.Any(),
				"hosts", c.config.Hosts, "database", c.config.Database, "type", "SELECT").
				Do(func(_ context.Context, _ string, value float64, _ ...string) { observed = value })

			var logged *Log

			mockLogger.EXPECT().Debug(gomock.Any()).Do(func(args ...any) { logged, _ = args[0].(*Log) })

			err := tc.call(t.Context(), &c)

			if tc.driverErr != nil {
				require.ErrorIs(t, err, tc.driverErr)
			} else {
				require.NoError(t, err)
			}

			floor := slowCall.Microseconds()

			assert.GreaterOrEqual(t, observed, float64(floor), "histogram must cover the driver call (µs)")

			require.NotNil(t, logged)
			assert.GreaterOrEqual(t, logged.Duration, floor, "debug log must cover the driver call (µs)")

			attr, ok := span.attrs[attribute.Key(fmt.Sprintf("clickhouse.%v.duration", tc.span))]
			require.True(t, ok, "span duration attribute must be set")
			assert.GreaterOrEqual(t, attr.AsInt64(), floor, "span attribute must cover the driver call (µs)")
			assert.Equal(t, 1, span.ended, "span must be ended exactly once")
		})
	}
}

func expectSlowExec(conn *MockConn, driverErr error) {
	conn.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, string, ...any) error {
			time.Sleep(slowCall)

			return driverErr
		})
}

func expectSlowSelect(dest any) func(conn *MockConn, driverErr error) {
	return func(conn *MockConn, driverErr error) {
		conn.EXPECT().Select(gomock.Any(), dest, gomock.Any(), gomock.Any()).
			DoAndReturn(func(context.Context, any, string, ...any) error {
				time.Sleep(slowCall)

				return driverErr
			})
	}
}

func expectSlowAsyncInsert(conn *MockConn, driverErr error) {
	conn.EXPECT().AsyncInsert(gomock.Any(), gomock.Any(), true, gomock.Any()).
		DoAndReturn(func(context.Context, string, bool, ...any) error {
			time.Sleep(slowCall)

			return driverErr
		})
}

func Test_ClickHouse_TypeLabelIgnoresQueryFormatting(t *testing.T) {
	mockConn, mockMetric, mockLogger, c := getClickHouseTestConnection(t)

	const query = "SELECT\n\tid\nFROM users"

	mockConn.EXPECT().Exec(gomock.Any(), query).Return(nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_clickhouse_stats", gomock.Any(),
		"hosts", c.config.Hosts, "database", c.config.Database, "type", "SELECT")

	require.NoError(t, c.Exec(t.Context(), query))
}

func Test_getOperationType(t *testing.T) {
	tests := []struct {
		query    string
		expected string
	}{
		{"INSERT INTO users VALUES (?)", "INSERT"},
		{"\tSELECT\n *", "SELECT"},
		{"\n\twith x as (select 1) select * from x", "WITH"},
		{"insert\tinto t", "INSERT"},
		{"select\r\nid from t", "SELECT"},
		{"", ""},
		{" \n\t", ""},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, getOperationType(tc.query), "query %q", tc.query)
	}
}

func Test_ClickHouse_ConcurrentExecEachRecordsItsOwnDuration(t *testing.T) {
	const callers = 16

	mockConn, mockMetric, mockLogger, c := getClickHouseTestConnection(t)

	mockConn.EXPECT().Exec(gomock.Any(), gomock.Any()).Times(callers).
		DoAndReturn(func(context.Context, string, ...any) error {
			time.Sleep(slowCall)

			return nil
		})
	mockLogger.EXPECT().Debug(gomock.Any()).Times(callers)

	var (
		mu       sync.Mutex
		observed []float64
	)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_clickhouse_stats", gomock.Any(),
		"hosts", c.config.Hosts, "database", c.config.Database, "type", "INSERT").Times(callers).
		Do(func(_ context.Context, _ string, value float64, _ ...string) {
			mu.Lock()
			defer mu.Unlock()

			observed = append(observed, value)
		})

	var wg sync.WaitGroup

	for range callers {
		wg.Go(func() {
			assert.NoError(t, c.Exec(t.Context(), "INSERT INTO users VALUES (1)"))
		})
	}

	wg.Wait()

	require.Len(t, observed, callers)

	for _, v := range observed {
		assert.GreaterOrEqual(t, v, float64(slowCall.Microseconds()), "each caller must record its own call (µs)")
	}
}

var errClickhouseOp = errors.New("clickhouse operation failed")

func Test_ClickHouse_UseTracer(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("gofr-clickhouse")

	tests := []struct {
		desc      string
		tracer    any
		expTracer trace.Tracer
	}{
		{desc: "valid tracer is set", tracer: tracer, expTracer: tracer},
		{desc: "non-tracer is ignored", tracer: "not a tracer", expTracer: nil},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			c := New(Config{})

			c.UseTracer(tc.tracer)

			assert.Equal(t, tc.expTracer, c.tracer)
		})
	}
}

func Test_ClickHouse_OperationsWithTracer(t *testing.T) {
	const query = "INSERT INTO users (id) VALUES (?)"

	tests := []struct {
		desc     string
		mockCall func(conn *MockConn)
		call     func(ctx context.Context, c *Client) error
		expErr   error
	}{
		{
			desc:     "exec",
			mockCall: func(conn *MockConn) { conn.EXPECT().Exec(gomock.Any(), query, "1").Return(nil) },
			call:     func(ctx context.Context, c *Client) error { return c.Exec(ctx, query, "1") },
		},
		{
			desc:     "exec error",
			mockCall: func(conn *MockConn) { conn.EXPECT().Exec(gomock.Any(), query, "1").Return(errClickhouseOp) },
			call:     func(ctx context.Context, c *Client) error { return c.Exec(ctx, query, "1") },
			expErr:   errClickhouseOp,
		},
		{
			desc:     "select error",
			mockCall: func(conn *MockConn) { conn.EXPECT().Select(gomock.Any(), nil, query, "1").Return(errClickhouseOp) },
			call:     func(ctx context.Context, c *Client) error { return c.Select(ctx, nil, query, "1") },
			expErr:   errClickhouseOp,
		},
		{
			desc:     "async insert",
			mockCall: func(conn *MockConn) { conn.EXPECT().AsyncInsert(gomock.Any(), query, false, "1").Return(nil) },
			call:     func(ctx context.Context, c *Client) error { return c.AsyncInsert(ctx, query, false, "1") },
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			mockConn, mockMetric, mockLogger, c := getClickHouseTestConnection(t)
			c.UseTracer(noop.NewTracerProvider().Tracer("gofr-clickhouse"))

			tc.mockCall(mockConn)
			mockLogger.EXPECT().Debug(gomock.Any())
			mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_clickhouse_stats", gomock.Any(), "hosts", c.config.Hosts,
				"database", c.config.Database, "type", "INSERT")

			err := tc.call(t.Context(), &c)

			require.ErrorIs(t, err, tc.expErr)
		})
	}
}
