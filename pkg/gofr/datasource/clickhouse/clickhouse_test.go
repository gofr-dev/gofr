package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	mockMetric.EXPECT().RecordHistogram(ctx, "app_clickhouse_stats", float64(0), "hosts", c.config.Hosts,
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

	mockMetric.EXPECT().RecordHistogram(ctx, "app_clickhouse_stats", float64(0), "hosts", c.config.Hosts,
		"database", c.config.Database, "type", "SELECT")

	err := c.Select(ctx, &user, "SELECT * FROM users")

	require.NoError(t, err)
}

func Test_ClickHouse_AsyncInsert(t *testing.T) {
	mockConn, mockMetric, mockLogger, c := getClickHouseTestConnection(t)

	ctx := context.Background()

	mockConn.EXPECT().AsyncInsert(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)", true,
		"8f165e2d-feef-416c-95f6-913ce3172e15", "user", "10").Return(nil)

	mockMetric.EXPECT().RecordHistogram(ctx, "app_clickhouse_stats", float64(0), "hosts", c.config.Hosts,
		"database", c.config.Database, "type", "INSERT")

	mockLogger.EXPECT().Debug(gomock.Any())

	err := c.AsyncInsert(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)", true,
		"8f165e2d-feef-416c-95f6-913ce3172e15", "user", "10")

	require.NoError(t, err)
}
