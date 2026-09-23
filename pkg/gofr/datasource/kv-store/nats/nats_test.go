package nats

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/mock/gomock"
)

var (
	errFailedToSet      = errors.New("failed to set")
	errConnectionFailed = errors.New("connection failed")
)

func Test_ClientSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockKV := NewMockKeyValue(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	configs := &Configs{
		Server: "nats://localhost:4222",
		Bucket: "test_bucket",
	}

	mockKV.EXPECT().
		Put("test_key", []byte("test_value")).
		Return(uint64(1), nil)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_nats_kv_stats",
		gomock.Any(),
		"bucket", configs.Bucket,
		"operation", "SET",
	).AnyTimes()

	cl := Client{
		kv:      mockKV,
		logger:  mockLogger,
		metrics: mockMetrics,
		configs: configs,
	}

	err := cl.Set(context.Background(), "test_key", "test_value")
	require.NoError(t, err)
}

func Test_ClientSetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockKV := NewMockKeyValue(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	configs := &Configs{
		Server: "nats://localhost:4222",
		Bucket: "test_bucket",
	}

	mockKV.EXPECT().
		Put("test_key", []byte("test_value")).
		Return(uint64(0), errFailedToSet)
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_nats_kv_stats",
		gomock.Any(),
		"bucket", configs.Bucket,
		"operation", "SET",
	).AnyTimes()

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	cl := Client{
		kv:      mockKV,
		logger:  mockLogger,
		metrics: mockMetrics,
		configs: configs,
	}

	err := cl.Set(context.Background(), "test_key", "test_value")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to set key-value pair")
}

func Test_ClientGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockKV := NewMockKeyValue(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	configs := &Configs{
		Server: "nats://localhost:4222",
		Bucket: "test_bucket",
	}

	mockEntry := &MockKeyValueEntry{value: []byte("test_value")}
	mockKV.EXPECT().
		Get("test_key").
		Return(mockEntry, nil)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_nats_kv_stats",
		gomock.Any(),
		"bucket", configs.Bucket,
		"operation", "GET",
	).AnyTimes()

	cl := Client{
		kv:      mockKV,
		logger:  mockLogger,
		metrics: mockMetrics,
		configs: configs,
	}

	val, err := cl.Get(context.Background(), "test_key")
	require.NoError(t, err)
	assert.Equal(t, "test_value", val)
}

func Test_ClientGetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockKV := NewMockKeyValue(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	configs := &Configs{
		Server: "nats://localhost:4222",
		Bucket: "test_bucket",
	}

	mockKV.EXPECT().
		Get("nonexistent_key").
		Return(nil, nats.ErrKeyNotFound)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_nats_kv_stats",
		gomock.Any(),
		"bucket", configs.Bucket,
		"operation", "GET",
	).AnyTimes()

	cl := Client{
		kv:      mockKV,
		logger:  mockLogger,
		metrics: mockMetrics,
		configs: configs,
	}

	val, err := cl.Get(context.Background(), "nonexistent_key")
	require.Error(t, err)
	assert.Empty(t, val)
	assert.Contains(t, err.Error(), "key not found")
}

func Test_ClientDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockKV := NewMockKeyValue(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	configs := &Configs{
		Server: "nats://localhost:4222",
		Bucket: "test_bucket",
	}

	mockKV.EXPECT().
		Delete("test_key").
		Return(nil)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_nats_kv_stats",
		gomock.Any(),
		"bucket", configs.Bucket,
		"operation", "DELETE",
	).AnyTimes()

	cl := Client{
		kv:      mockKV,
		logger:  mockLogger,
		metrics: mockMetrics,
		configs: configs,
	}

	err := cl.Delete(context.Background(), "test_key")
	require.NoError(t, err)
}

func Test_ClientDeleteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockKV := NewMockKeyValue(ctrl)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	configs := &Configs{
		Server: "nats://localhost:4222",
		Bucket: "test_bucket",
	}

	mockKV.EXPECT().
		Delete("nonexistent_key").
		Return(nats.ErrKeyNotFound)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(),
		"app_nats_kv_stats",
		gomock.Any(),
		"bucket", configs.Bucket,
		"operation", "DELETE",
	).AnyTimes()

	cl := Client{
		kv:      mockKV,
		logger:  mockLogger,
		metrics: mockMetrics,
		configs: configs,
	}

	err := cl.Delete(context.Background(), "nonexistent_key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")
}

func Test_ClientHealthCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJts(ctrl)
	mockLogger := NewMockLogger(ctrl)

	configs := &Configs{
		Server: "nats://localhost:4222",
		Bucket: "test_bucket",
	}

	mockJS.EXPECT().
		AccountInfo().
		Return(&nats.AccountInfo{}, nil)

	mockLogger.EXPECT().
		Debug(gomock.Any()).
		Do(func(log *Log) {
			assert.Equal(t, "HEALTH CHECK", log.Type)
			assert.Equal(t, "health", log.Key)
			assert.Equal(t, fmt.Sprintf("Checking connection status for bucket '%s' at '%s'",
				configs.Bucket, configs.Server), log.Value)
		}).
		Times(1)

	cl := Client{
		js:      mockJS,
		logger:  mockLogger,
		configs: configs,
	}

	val, err := cl.HealthCheck(context.Background())
	require.NoError(t, err)

	health := val.(*Health)
	assert.Equal(t, "UP", health.Status)
	assert.Equal(t, configs.Server, health.Details["url"])
	assert.Equal(t, configs.Bucket, health.Details["bucket"])
}

func Test_ClientHealthCheckFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJts(ctrl)
	mockLogger := NewMockLogger(ctrl)

	configs := &Configs{
		Server: "nats://localhost:4222",
		Bucket: "test_bucket",
	}

	mockJS.EXPECT().
		AccountInfo().
		Return(nil, errConnectionFailed)

	// Mock the Debug call for failed health check
	mockLogger.EXPECT().
		Debug(gomock.Any()).
		Do(func(log *Log) {
			assert.Equal(t, "HEALTH CHECK", log.Type)
			assert.Equal(t, "health", log.Key)
			assert.Equal(t, fmt.Sprintf("Connection failed for bucket '%s' at '%s'",
				configs.Bucket, configs.Server), log.Value)
		}).
		Times(1)

	cl := Client{
		js:      mockJS,
		logger:  mockLogger,
		configs: configs,
	}

	val, err := cl.HealthCheck(context.Background())
	require.Error(t, err)
	require.Equal(t, errStatusDown, err)

	health := val.(*Health)
	assert.Equal(t, "DOWN", health.Status)
	assert.Equal(t, configs.Server, health.Details["url"])
	assert.Equal(t, configs.Bucket, health.Details["bucket"])
}

var errKVFailure = errors.New("kv failure")

// startFakeNATSServer starts an in-process TCP listener that speaks the minimal
// NATS handshake (INFO, CONNECT, PING/PONG) so that nats.Connect succeeds without a real server.
func startFakeNATSServer(t *testing.T) string {
	t.Helper()

	var lc net.ListenConfig

	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			go serveFakeNATSConn(conn)
		}
	}()

	return "nats://" + ln.Addr().String()
}

func serveFakeNATSConn(conn net.Conn) {
	defer conn.Close()

	_, err := conn.Write([]byte(`INFO {"server_id":"fake","version":"2.10.0","proto":1,"max_payload":1048576}` + "\r\n"))
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "PING") {
			if _, err := conn.Write([]byte("PONG\r\n")); err != nil {
				return
			}
		}
	}
}

// closedServerAddress returns the address of a listener that has already been closed,
// so connecting to it fails immediately with connection refused.
func closedServerAddress(t *testing.T) string {
	t.Helper()

	var lc net.ListenConfig

	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	return "nats://" + addr
}

func Test_ClientConnect(t *testing.T) {
	tests := []struct {
		desc       string
		server     func(t *testing.T) string
		bucket     string
		setupMocks func(l *MockLogger)
		expConn    bool
		expJS      bool
	}{
		{
			desc:   "connection to NATS fails",
			server: closedServerAddress,
			bucket: "test_bucket",
			setupMocks: func(l *MockLogger) {
				l.EXPECT().Debugf("connecting to NATS-KV Store at %v with bucket %q", gomock.Any(), "test_bucket")
				l.EXPECT().Errorf("error while connecting to NATS: %v", gomock.Any())
			},
		},
		{
			desc:   "KV bucket creation fails for invalid bucket name",
			server: startFakeNATSServer,
			bucket: "invalid bucket!",
			setupMocks: func(l *MockLogger) {
				l.EXPECT().Debugf("connecting to NATS-KV Store at %v with bucket %q", gomock.Any(), "invalid bucket!")
				l.EXPECT().Debug("connection to NATS successful")
				l.EXPECT().Debug("jetStream initialized successfully")
				l.EXPECT().Errorf("error while creating/accessing KV bucket: %v", nats.ErrInvalidBucketName)
			},
			expConn: true,
			expJS:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockLogger := NewMockLogger(ctrl)
			mockMetrics := NewMockMetrics(ctrl)

			tc.setupMocks(mockLogger)
			mockMetrics.EXPECT().NewHistogram("app_nats_kv_stats",
				"Response time of NATS KV operations in milliseconds.", gomock.Any())

			cl := New(Configs{Server: tc.server(t), Bucket: tc.bucket})
			cl.UseLogger(mockLogger)
			cl.UseMetrics(mockMetrics)

			cl.Connect()
			t.Cleanup(cl.conn.Close)

			assert.Equal(t, tc.expConn, cl.conn != nil)
			assert.Equal(t, tc.expJS, cl.js != nil)
			assert.Nil(t, cl.kv)
		})
	}
}

func Test_JetStreamAccountInfo(t *testing.T) {
	tests := []struct {
		desc   string
		expErr error
	}{
		{
			desc:   "account info on closed connection returns error",
			expErr: nats.ErrConnectionClosed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			nc, err := nats.Connect(startFakeNATSServer(t))
			require.NoError(t, err)

			js, err := nc.JetStream()
			require.NoError(t, err)

			nc.Close()

			info, err := jetStream{js}.AccountInfo()

			require.ErrorIs(t, err, tc.expErr)
			assert.Nil(t, info)
		})
	}
}

func Test_ClientUseMethods(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	tracer := noop.NewTracerProvider().Tracer("test")

	tests := []struct {
		desc       string
		logger     any
		metrics    any
		tracer     any
		expLogger  Logger
		expMetrics Metrics
		expTracer  trace.Tracer
	}{
		{
			desc:       "valid logger, metrics and tracer are set",
			logger:     mockLogger,
			metrics:    mockMetrics,
			tracer:     tracer,
			expLogger:  mockLogger,
			expMetrics: mockMetrics,
			expTracer:  tracer,
		},
		{
			desc:    "invalid types are ignored",
			logger:  "logger",
			metrics: 1,
			tracer:  struct{}{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			cl := New(Configs{Server: "nats://localhost:4222", Bucket: "test_bucket"})

			cl.UseLogger(tc.logger)
			cl.UseMetrics(tc.metrics)
			cl.UseTracer(tc.tracer)

			assert.Equal(t, &Configs{Server: "nats://localhost:4222", Bucket: "test_bucket"}, cl.configs)
			assert.Equal(t, tc.expLogger, cl.logger)
			assert.Equal(t, tc.expMetrics, cl.metrics)
			assert.Equal(t, tc.expTracer, cl.tracer)
		})
	}
}

func Test_ClientOperationsWithTracer(t *testing.T) {
	configs := &Configs{Server: "nats://localhost:4222", Bucket: "test_bucket"}

	tests := []struct {
		desc      string
		setupMock func(kv *MockKeyValue)
		operation func(ctx context.Context, cl *Client) (string, error)
		expVal    string
		expErr    error
	}{
		{
			desc: "get with tracer",
			setupMock: func(kv *MockKeyValue) {
				kv.EXPECT().Get("test_key").Return(&MockKeyValueEntry{value: []byte("test_value")}, nil)
			},
			operation: func(ctx context.Context, cl *Client) (string, error) { return cl.Get(ctx, "test_key") },
			expVal:    "test_value",
		},
		{
			desc: "get returns generic error",
			setupMock: func(kv *MockKeyValue) {
				kv.EXPECT().Get("test_key").Return(nil, errKVFailure)
			},
			operation: func(ctx context.Context, cl *Client) (string, error) { return cl.Get(ctx, "test_key") },
			expErr:    errKVFailure,
		},
		{
			desc: "set with tracer",
			setupMock: func(kv *MockKeyValue) {
				kv.EXPECT().Put("test_key", []byte("test_value")).Return(uint64(1), nil)
			},
			operation: func(ctx context.Context, cl *Client) (string, error) {
				return "", cl.Set(ctx, "test_key", "test_value")
			},
		},
		{
			desc: "delete with tracer",
			setupMock: func(kv *MockKeyValue) {
				kv.EXPECT().Delete("test_key").Return(nil)
			},
			operation: func(ctx context.Context, cl *Client) (string, error) { return "", cl.Delete(ctx, "test_key") },
		},
		{
			desc: "delete returns generic error",
			setupMock: func(kv *MockKeyValue) {
				kv.EXPECT().Delete("test_key").Return(errKVFailure)
			},
			operation: func(ctx context.Context, cl *Client) (string, error) { return "", cl.Delete(ctx, "test_key") },
			expErr:    errKVFailure,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockKV := NewMockKeyValue(ctrl)
			mockLogger := NewMockLogger(ctrl)
			mockMetrics := NewMockMetrics(ctrl)

			tc.setupMock(mockKV)
			mockLogger.EXPECT().Debug(gomock.Any())
			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_nats_kv_stats", gomock.Any(),
				"bucket", configs.Bucket, "operation", gomock.Any())

			cl := &Client{
				kv:      mockKV,
				logger:  mockLogger,
				metrics: mockMetrics,
				configs: configs,
				tracer:  noop.NewTracerProvider().Tracer("test"),
			}

			val, err := tc.operation(t.Context(), cl)

			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expVal, val)
		})
	}
}

func Test_ClientHealthCheckWithTracer(t *testing.T) {
	configs := &Configs{Server: "nats://localhost:4222", Bucket: "test_bucket"}

	tests := []struct {
		desc      string
		infoErr   error
		expStatus string
		expErr    error
	}{
		{desc: "health check up with tracer", expStatus: "UP"},
		{desc: "health check down with tracer", infoErr: errConnectionFailed, expStatus: "DOWN", expErr: errStatusDown},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockJS := NewMockJts(ctrl)
			mockLogger := NewMockLogger(ctrl)

			mockJS.EXPECT().AccountInfo().Return(&nats.AccountInfo{}, tc.infoErr)
			mockLogger.EXPECT().Debug(gomock.Any())

			cl := Client{
				js:      mockJS,
				logger:  mockLogger,
				configs: configs,
				tracer:  noop.NewTracerProvider().Tracer("test"),
			}

			val, err := cl.HealthCheck(t.Context())

			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expStatus, val.(*Health).Status)
		})
	}
}

func Test_LogPrettyPrint(t *testing.T) {
	uuidKey := "123e4567-e89b-12d3-a456-426614174000"

	tests := []struct {
		desc     string
		log      Log
		expected string
	}{
		{
			desc:     "GET operation",
			log:      Log{Type: "GET", Duration: 10, Key: "key1", Value: "bucket"},
			expected: "Fetching record from bucket 'bucket' with ID 'key1'",
		},
		{
			desc:     "SET operation with UUID key",
			log:      Log{Type: "SET", Duration: 10, Key: uuidKey, Value: "bucket"},
			expected: "Creating new record in bucket 'bucket' with ID '" + uuidKey + "'",
		},
		{
			desc:     "SET operation with non UUID key",
			log:      Log{Type: "SET", Duration: 10, Key: "key1", Value: "bucket"},
			expected: "Updating record with ID 'key1' in bucket 'bucket'",
		},
		{
			desc:     "DELETE operation",
			log:      Log{Type: "DELETE", Duration: 10, Key: "key1", Value: "bucket"},
			expected: "Deleting record from bucket 'bucket' with ID 'key1'",
		},
		{
			desc:     "unknown operation",
			log:      Log{Type: "HEALTH CHECK", Duration: 25},
			expected: "HEALTH CHECK",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var buf bytes.Buffer

			tc.log.PrettyPrint(&buf)

			assert.Contains(t, buf.String(), tc.expected)
			assert.Contains(t, buf.String(), "NATS")
			assert.Contains(t, buf.String(), fmt.Sprintf("%dμs", tc.log.Duration))
		})
	}
}
