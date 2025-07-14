package oracle

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func getOracleTestConnection(t *testing.T) (*MockConn, *MockMetrics, *MockLogger, Client) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockConn := NewMockConn(ctrl)
	mockMetric := NewMockMetrics(ctrl)
	mockLogger := NewMockLogger(ctrl)
	c := Client{conn: mockConn, config: Config{
		Host: "localhost", Port: 1521, Username: "system", Password: "password", Service: "FREEPDB1",
	}, logger: mockLogger, metrics: mockMetric}
	return mockConn, mockMetric, mockLogger, c
}

func Test_Oracle_HealthUP(t *testing.T) {
	mockConn, _, _, c := getOracleTestConnection(t)
	mockConn.EXPECT().Ping(gomock.Any()).Return(nil)
	resp, _ := c.HealthCheck(context.Background())
	health, ok := resp.(*Health)
	require.True(t, ok)
	assert.Equal(t, "UP", health.Status)
}

func Test_Oracle_HealthDOWN(t *testing.T) {
	mockConn, _, _, c := getOracleTestConnection(t)
	mockConn.EXPECT().Ping(gomock.Any()).Return(sql.ErrConnDone)
	resp, err := c.HealthCheck(context.Background())
	require.ErrorIs(t, err, errStatusDown)
	health, ok := resp.(*Health)
	require.True(t, ok)
	assert.Equal(t, "DOWN", health.Status)
}

func Test_Oracle_Exec(t *testing.T) {
	mockConn, _, mockLogger, c := getOracleTestConnection(t)
	ctx := context.Background()
	mockConn.EXPECT().Exec(ctx, "INSERT INTO users (id, name) VALUES (?, ?)", 1, "user").Return(nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	// Metrics can be asserted if implemented
	err := c.Exec(ctx, "INSERT INTO users (id, name) VALUES (?, ?)", 1, "user")
	require.NoError(t, err)
}

func Test_Oracle_Select(t *testing.T) {
	mockConn, _, mockLogger, c := getOracleTestConnection(t)
	type User struct {
		ID   int
		Name string
	}
	ctx := context.Background()
	var users []User
	mockConn.EXPECT().Select(ctx, &users, "SELECT * FROM users").Return(nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	err := c.Select(ctx, &users, "SELECT * FROM users")
	require.NoError(t, err)
}

func Test_Oracle_AsyncInsert(t *testing.T) {
	mockConn, _, mockLogger, c := getOracleTestConnection(t)
	ctx := context.Background()
	mockConn.EXPECT().AsyncInsert(ctx, "INSERT INTO users (id, name) VALUES (?, ?)", true, 1, "user").Return(nil)
	mockLogger.EXPECT().Debug(gomock.Any())
	err := c.AsyncInsert(ctx, "INSERT INTO users (id, name) VALUES (?, ?)", true, 1, "user")
	require.NoError(t, err)
}
