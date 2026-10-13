package sql

import (
	"context"
	"database/sql/driver"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
)

var (
	errFakeNoConn  = errors.New("fake: connection not available in tests")
	errFakeCleanup = errors.New("connector cleanup failed")
)

// fakeConnector is a driver.Connector whose connections always fail. It lets tests
// drive NewSQLFromConnector's wrapping, cleanup and config handling without a live
// database — the connector dial itself is the only GCP-gated part.
type fakeConnector struct{}

func (fakeConnector) Connect(context.Context) (driver.Conn, error) { return nil, errFakeNoConn }
func (fakeConnector) Driver() driver.Driver                        { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return nil, errFakeNoConn }

// blockingPingConnector hands out connections whose first ping (the connect check)
// succeeds and whose later pings block until their context is done. It lets a test
// hold the background retry goroutine mid-ping while Close runs.
type blockingPingConnector struct {
	pings   atomic.Int32
	active  atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (c *blockingPingConnector) Connect(context.Context) (driver.Conn, error) {
	return blockingPingConn{c: c}, nil
}

func (*blockingPingConnector) Driver() driver.Driver { return fakeDriver{} }

type blockingPingConn struct{ c *blockingPingConnector }

func (blockingPingConn) Prepare(string) (driver.Stmt, error) { return nil, errFakeNoConn }
func (blockingPingConn) Close() error                        { return nil }
func (blockingPingConn) Begin() (driver.Tx, error)           { return nil, errFakeNoConn }

func (b blockingPingConn) Ping(ctx context.Context) error {
	if b.c.pings.Add(1) == 1 {
		return nil
	}

	b.c.active.Add(1)
	defer b.c.active.Add(-1)

	select {
	case b.c.started <- struct{}{}:
	default:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-b.c.release:
		return nil
	}
}

func newConnectorMetrics(t *testing.T) Metrics {
	t.Helper()

	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	metrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	metrics.EXPECT().SetGauge(gomock.Any(), gomock.Any()).AnyTimes()

	return metrics
}

func TestNewSQLFromConnector_NilConnector(t *testing.T) {
	got := NewSQLFromConnector(nil, config.NewMockConfig(nil),
		logging.NewMockLogger(logging.DEBUG), newConnectorMetrics(t), nil)

	assert.Nil(t, got, "a nil connector must return a nil datasource")
}

func TestNewSQLFromConnector_WrapsConnector(t *testing.T) {
	conf := config.NewMockConfig(map[string]string{
		"DB_DIALECT": dialectPostgres,
		"DB_HOST":    "proj:region:inst",
		"DB_NAME":    "app",
		"DB_USER":    "app-sa@proj.iam",
		"DB_PORT":    "5432",
	})

	got := NewSQLFromConnector(fakeConnector{}, conf,
		logging.NewMockLogger(logging.DEBUG), newConnectorMetrics(t), nil)

	require.NotNil(t, got)
	t.Cleanup(func() { _ = got.Close() })

	assert.Equal(t, dialectPostgres, got.Dialect(), "dialect label comes from config")
	// The connector owns routing, so the port label is cleared even when DB_PORT is
	// set — this is what keeps connection logs from printing a dangling ':'.
	assert.Empty(t, got.config.Port, "connector path must clear the port label")
	assert.NotNil(t, got.HealthCheck(), "wrapped connector exposes health checks")
}

// TestNewSQLFromConnector_CloseRunsCleanup covers the IAM-path teardown: Close closes
// the wrapped pool and runs the connector cleanup, joining both errors, and the
// cleanup runs exactly once.
func TestNewSQLFromConnector_CloseRunsCleanup(t *testing.T) {
	calls := 0
	cleanup := func() error {
		calls++
		return errFakeCleanup
	}

	got := NewSQLFromConnector(fakeConnector{}, config.NewMockConfig(map[string]string{"DB_DIALECT": dialectPostgres}),
		logging.NewMockLogger(logging.DEBUG), newConnectorMetrics(t), cleanup)
	require.NotNil(t, got)

	err := got.Close()

	require.ErrorIs(t, err, errFakeCleanup, "Close must surface the connector cleanup error")
	assert.Equal(t, 1, calls, "cleanup runs exactly once")

	require.NoError(t, got.Close(), "cleared cleanup is a no-op on a second Close")
	assert.Equal(t, 1, calls, "cleanup is not invoked again")
}

// TestNewSQLFromConnector_NilCleanup verifies Close is safe when no cleanup was set
// (the path a provider without teardown would take).
func TestNewSQLFromConnector_NilCleanup(t *testing.T) {
	got := NewSQLFromConnector(fakeConnector{}, config.NewMockConfig(map[string]string{"DB_DIALECT": dialectPostgres}),
		logging.NewMockLogger(logging.DEBUG), newConnectorMetrics(t), nil)
	require.NotNil(t, got)

	assert.NoError(t, got.Close())
}

// TestDB_Close_WaitsForBackgroundPing pins Close's contract: it cancels a background
// ping that is in flight and waits for the retry and metrics goroutines to exit, so
// nothing touches the database after Close returns.
func TestDB_Close_WaitsForBackgroundPing(t *testing.T) {
	leakOpts := goleak.IgnoreCurrent()

	connector := &blockingPingConnector{started: make(chan struct{}, 1), release: make(chan struct{})}
	// Unblocks the ping if Close failed to cancel it, so a red run leaks nothing.
	t.Cleanup(func() { close(connector.release) })

	got := NewSQLFromConnector(connector, config.NewMockConfig(map[string]string{"DB_DIALECT": dialectPostgres}),
		logging.NewMockLogger(logging.DEBUG), newConnectorMetrics(t), nil)
	require.NotNil(t, got)

	select {
	case <-connector.started:
	case <-time.After(5 * time.Second):
		t.Fatal("background ping never started")
	}

	closed := make(chan error, 1)

	go func() { closed <- got.Close() }()

	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Close blocked on an in-flight background ping")
	}

	assert.Zero(t, connector.active.Load(), "a background ping was still running after Close returned")
	goleak.VerifyNone(t, leakOpts)
}
