package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"
)

func getClickHouseTestConnection(t *testing.T) (*MockConn, *MockMetrics, client) {
	t.Helper()

	ctrl := gomock.NewController(t)

	mockConn := NewMockConn(ctrl)
	mockMetric := NewMockMetrics(ctrl)

	c := client{conn: mockConn, config: Config{
		Hosts:    "localhost",
		Username: "user",
		Password: "pass",
		Database: "test",
	}, logger: NewMockLogger(DEBUG), metrics: mockMetric}

	return mockConn, mockMetric, c
}

func Test_ClickHouse_ConnectAndMetricRegistrationAndPingFailure(t *testing.T) {
	logs := stderrOutputForFunc(func() {
		_, mockMetric, _ := getClickHouseTestConnection(t)
		mockLogger := NewMockLogger(DEBUG)

		cl := New(Config{
			Hosts:    "localhost:8000",
			Username: "user",
			Password: "pass",
			Database: "test",
		})

		cl.UseLogger(mockLogger)
		cl.UseMetrics(mockMetric)

		mockMetric.EXPECT().NewHistogram("app_clickhouse_stats", "Response time of Clickhouse queries in milliseconds.", gomock.Any())
		mockMetric.EXPECT().NewGauge("app_clickhouse_open_connections", "Number of open Clickhouse connections.")
		mockMetric.EXPECT().NewGauge("app_clickhouse_idle_connections", "Number of idle Clickhouse connections.")
		mockMetric.EXPECT().SetGauge("app_clickhouse_open_connections", gomock.Any()).AnyTimes()
		mockMetric.EXPECT().SetGauge("app_clickhouse_idle_connections", gomock.Any()).AnyTimes()

		cl.Connect()

		time.Sleep(1 * time.Second)
	})

	assert.Contains(t, logs, "ping failed with error dial tcp [::1]:8000: connect: connection refused")
}

func stderrOutputForFunc(f func()) string {
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w

	f()

	_ = w.Close()

	out, _ := io.ReadAll(r)
	os.Stderr = old

	return string(out)
}

func Test_ClickHouse_HealthUP(t *testing.T) {
	mockConn, _, c := getClickHouseTestConnection(t)

	mockConn.EXPECT().Ping(gomock.Any()).Return(nil)

	resp := c.HealthCheck()

	assert.Contains(t, fmt.Sprint(resp), "UP")
}

func Test_ClickHouse_HealthDOWN(t *testing.T) {
	mockConn, _, c := getClickHouseTestConnection(t)

	mockConn.EXPECT().Ping(gomock.Any()).Return(sql.ErrConnDone)

	resp := c.HealthCheck()

	assert.Contains(t, fmt.Sprint(resp), "DOWN")
}

func Test_ClickHouse_Exec(t *testing.T) {
	mockConn, mockMetric, c := getClickHouseTestConnection(t)

	ctx := context.Background()

	mockConn.EXPECT().Exec(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)",
		"8f165e2d-feef-416c-95f6-913ce3172e15", "gofr", "10").Return(nil)

	mockMetric.EXPECT().RecordHistogram(ctx, "app_clickhouse_stats", float64(0), "hosts", c.config.Hosts,
		"database", c.config.Database, "type", "INSERT")

	err := c.Exec(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)", "8f165e2d-feef-416c-95f6-913ce3172e15", "gofr", "10")

	assert.Nil(t, err)
}

func Test_ClickHouse_Select(t *testing.T) {
	mockConn, mockMetric, c := getClickHouseTestConnection(t)

	type User struct {
		ID   string `ch:"id"`
		Name string `ch:"name"`
		Age  string `ch:"age"`
	}

	ctx := context.Background()

	var user []User

	mockConn.EXPECT().Select(ctx, &user, "SELECT * FROM users").Return(nil)

	mockMetric.EXPECT().RecordHistogram(ctx, "app_clickhouse_stats", float64(0), "hosts", c.config.Hosts,
		"database", c.config.Database, "type", "SELECT")

	err := c.Select(ctx, &user, "SELECT * FROM users")

	assert.Nil(t, err)
}

func Test_ClickHouse_AsyncInsert(t *testing.T) {
	mockConn, mockMetric, c := getClickHouseTestConnection(t)

	ctx := context.Background()

	mockConn.EXPECT().AsyncInsert(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)", true,
		"8f165e2d-feef-416c-95f6-913ce3172e15", "user", "10").Return(nil)

	mockMetric.EXPECT().RecordHistogram(ctx, "app_clickhouse_stats", float64(0), "hosts", c.config.Hosts,
		"database", c.config.Database, "type", "INSERT")

	err := c.AsyncInsert(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)", true,
		"8f165e2d-feef-416c-95f6-913ce3172e15", "user", "10")

	assert.Nil(t, err)
}
