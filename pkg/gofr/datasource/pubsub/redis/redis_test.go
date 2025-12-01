package redis

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.opentelemetry.io/otel/trace/noop"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
	"sync"
)

var (
	//nolint:gochecknoglobals // used for testing purposes only
	testMessage = []byte("test message")
	testTopic   = "test-topic"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		validate func(t *testing.T, client *Client)
	}{
		{
			name: "with config",
			cfg:  DefaultConfig(),
			validate: func(t *testing.T, client *Client) {
				require.NotNil(t, client)
				assert.NotNil(t, client.cfg)
				assert.NotNil(t, client.receiveChan)
				assert.NotNil(t, client.subStarted)
				assert.NotNil(t, client.subCancel)
				assert.NotNil(t, client.subPubSub)
				assert.NotNil(t, client.subWg)
				assert.NotNil(t, client.chanClosed)
			},
		},
		{
			name: "nil config",
			cfg:  nil,
			validate: func(t *testing.T, client *Client) {
				require.NotNil(t, client)
				assert.NotNil(t, client.cfg)
				assert.Equal(t, DefaultConfig().Addr, client.cfg.Addr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New(tt.cfg)
			tt.validate(t, client)
		})
	}
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func() config.Config
		expectNil   bool
		validate    func(t *testing.T, client *Client)
	}{
		{
			name: "with redis address",
			setupConfig: func() config.Config {
				mockConfig := config.NewMockConfig(map[string]string{
					"REDIS_PUBSUB_ADDR": "localhost:6379",
				})
				return mockConfig
			},
			expectNil: false,
			validate: func(t *testing.T, client *Client) {
				require.NotNil(t, client)
				assert.NotNil(t, client.pubConn)
				assert.NotNil(t, client.subConn)
				assert.NotNil(t, client.queryConn)
			},
		},
		{
			name: "without redis address",
			setupConfig: func() config.Config {
				// Empty config - getRedisPubSubConfig will return DefaultConfig with "localhost:6379"
				// So client will be created, not nil
				mockConfig := config.NewMockConfig(map[string]string{})
				return mockConfig
			},
			expectNil: false, // DefaultConfig has address, so client is created
			validate: func(t *testing.T, client *Client) {
				require.NotNil(t, client)
				assert.Equal(t, "localhost:6379", client.cfg.Addr)
			},
		},
		{
			name: "with redis host and port",
			setupConfig: func() config.Config {
				mockConfig := config.NewMockConfig(map[string]string{
					"REDIS_HOST": "localhost",
					"REDIS_PORT": "6380",
				})
				return mockConfig
			},
			expectNil: false,
			validate: func(t *testing.T, client *Client) {
				require.NotNil(t, client)
				assert.Equal(t, "localhost:6380", client.cfg.Addr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConfig := tt.setupConfig()
			mockLogger := logging.NewMockLogger(logging.DEBUG)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
			mockMetrics := NewMockMetrics(ctrl)

			client := NewClient(mockConfig, mockLogger, mockMetrics)

			if tt.expectNil {
				assert.Nil(t, client)
			} else {
				tt.validate(t, client)
				require.NoError(t, client.Close())
			}
		})
	}
}

func TestUseLogger(t *testing.T) {
	tests := []struct {
		name     string
		logger   any
		validate func(t *testing.T, client *Client)
	}{
		{
			name:   "with redis.Logger",
			logger: func() Logger {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				return NewMockLogger(ctrl)
			}(),
			validate: func(t *testing.T, client *Client) {
	assert.NotNil(t, client.logger)
			},
		},
		{
			name:   "with pubsub.Logger",
			logger: &mockPubSubLogger{},
			validate: func(t *testing.T, client *Client) {
				assert.NotNil(t, client.logger)
			},
		},
		{
			name:   "with nil logger",
			logger: nil,
			validate: func(t *testing.T, client *Client) {
				assert.Nil(t, client.logger)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
	client := New(DefaultConfig())
			client.UseLogger(tt.logger)
			tt.validate(t, client)
		})
	}
}

func TestUseMetrics(t *testing.T) {
	tests := []struct {
		name     string
		metrics  any
		validate func(t *testing.T, client *Client)
	}{
		{
			name: "with Metrics interface",
			metrics: func() Metrics {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
				return NewMockMetrics(ctrl)
			}(),
			validate: func(t *testing.T, client *Client) {
				assert.NotNil(t, client.metrics)
			},
		},
		{
			name:   "with nil metrics",
			metrics: nil,
			validate: func(t *testing.T, client *Client) {
				assert.Nil(t, client.metrics)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
	client := New(DefaultConfig())
			client.UseMetrics(tt.metrics)
			tt.validate(t, client)
		})
	}
}

func TestUseTracer(t *testing.T) {
	tests := []struct {
		name     string
		tracer   any
		validate func(t *testing.T, client *Client)
	}{
		{
			name:   "with trace.Tracer",
			tracer: noop.NewTracerProvider().Tracer("test"),
			validate: func(t *testing.T, client *Client) {
	assert.NotNil(t, client.tracer)
			},
		},
		{
			name:   "with nil tracer",
			tracer: nil,
			validate: func(t *testing.T, client *Client) {
				assert.Nil(t, client.tracer)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New(DefaultConfig())
			client.UseTracer(tt.tracer)
			tt.validate(t, client)
		})
	}
}

func TestConnect(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		validate    func(t *testing.T, client *Client)
	}{
		{
			name: "successful connection",
			setupClient: func(t *testing.T) (*Client, func()) {
	s, err := miniredis.Run()
	require.NoError(t, err)

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client.UseLogger(mockLogger)

	client.Connect()
	time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			validate: func(t *testing.T, client *Client) {
	assert.NotNil(t, client.pubConn)
	assert.NotNil(t, client.subConn)
	assert.NotNil(t, client.queryConn)
			},
		},
		{
			name: "invalid config - empty address",
			setupClient: func(t *testing.T) (*Client, func()) {
				ctrl := gomock.NewController(t)
				mockLogger := NewMockLogger(ctrl)
				mockLogger.EXPECT().Errorf("could not initialize Redis, error: %v", gomock.Any())

				cfg := &Config{
					Addr: "",
					DB:   -1,
				}

				client := New(cfg)
				client.UseLogger(mockLogger)
				client.Connect()

				return client, func() {
					ctrl.Finish()
				}
			},
			validate: func(t *testing.T, client *Client) {
				assert.Nil(t, client.pubConn)
				assert.Nil(t, client.subConn)
			},
		},
		{
			name: "invalid config - invalid DB",
			setupClient: func(t *testing.T) (*Client, func()) {
				ctrl := gomock.NewController(t)
	mockLogger := NewMockLogger(ctrl)
	mockLogger.EXPECT().Errorf("could not initialize Redis, error: %v", gomock.Any())

	cfg := &Config{
					Addr: "localhost:6379",
					DB:   -1,
	}

	client := New(cfg)
	client.UseLogger(mockLogger)
	client.Connect()

				return client, func() {
					ctrl.Finish()
				}
			},
			validate: func(t *testing.T, client *Client) {
	assert.Nil(t, client.pubConn)
	assert.Nil(t, client.subConn)
			},
		},
		{
			name: "connection failure - invalid address",
			setupClient: func(t *testing.T) (*Client, func()) {
				// Use real logger to avoid goroutine issues with mocks
				mockLogger := logging.NewMockLogger(logging.DEBUG)

				cfg := &Config{
					Addr:        "invalid:6379",
					DB:          0,
					DialTimeout: 100 * time.Millisecond,
				}

				client := New(cfg)
				client.UseLogger(mockLogger)
				client.Connect()

				return client, func() {
					// Close immediately to stop retry goroutine
					_ = client.Close()
				}
			},
			validate: func(t *testing.T, client *Client) {
				// Connections are created even if they fail
				// The retry mechanism runs in background
				assert.NotNil(t, client.pubConn) // Client is created, connection just fails
			},
		},
		{
			name: "query connection ping failure",
			setupClient: func(t *testing.T) (*Client, func()) {
	s, err := miniredis.Run()
	require.NoError(t, err)

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client.UseLogger(mockLogger)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				// Close query connection to simulate ping failure
				_ = client.queryConn.Close()

				// Call Connect again to trigger query connection ping test
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			validate: func(t *testing.T, client *Client) {
				// Query connection should be recreated
				assert.NotNil(t, client.queryConn)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()
			tt.validate(t, client)
		})
	}
}

func TestPublish(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		topic       string
		message     []byte
		setupMocks  func(t *testing.T, client *Client)
		wantErr     bool
		validateErr func(t *testing.T, err error)
	}{
		{
			name: "successful publish",
			setupClient: func(t *testing.T) (*Client, func()) {
	s, err := miniredis.Run()
	require.NoError(t, err)

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client.UseLogger(mockLogger)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:   testTopic,
			message: testMessage,
			setupMocks: func(t *testing.T, client *Client) {
	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	client.UseMetrics(mockMetrics)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", testTopic)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_success_count", "topic", testTopic)
			},
			wantErr: false,
		},
		{
			name: "no connection",
			setupClient: func(t *testing.T) (*Client, func()) {
				return New(DefaultConfig()), func() {}
			},
			topic:   testTopic,
			message: testMessage,
			setupMocks: func(t *testing.T, client *Client) {},
			wantErr: true,
			validateErr: func(t *testing.T, err error) {
				assert.Equal(t, errPublisherNotConfigured, err)
			},
		},
		{
			name: "empty topic",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
	require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:   "",
			message: testMessage,
			setupMocks: func(t *testing.T, client *Client) {},
			wantErr: true,
			validateErr: func(t *testing.T, err error) {
				assert.Equal(t, errPublisherNotConfigured, err)
			},
		},
		{
			name: "publish with disconnected client",
			setupClient: func(t *testing.T) (*Client, func()) {
	s, err := miniredis.Run()
	require.NoError(t, err)

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	client.Connect()
	time.Sleep(100 * time.Millisecond)
				s.Close() // Close Redis to simulate disconnection

				return client, func() {
					_ = client.Close()
				}
			},
			topic:   testTopic,
			message: testMessage,
			setupMocks: func(t *testing.T, client *Client) {
				ctrl := gomock.NewController(t)
				mockMetrics := NewMockMetrics(ctrl)
				client.UseMetrics(mockMetrics)
				mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", testTopic)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()
			tt.setupMocks(t, client)

	ctx := context.Background()
			err := client.Publish(ctx, tt.topic, tt.message)

			if tt.wantErr {
	require.Error(t, err)
				if tt.validateErr != nil {
					tt.validateErr(t, err)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSubscribe(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		topic       string
		setupMocks  func(t *testing.T, client *Client)
		publishMsg  bool
		wantErr     bool
		validateErr func(t *testing.T, err error)
		validateMsg func(t *testing.T, msg *pubsub.Message)
	}{
		{
			name: "successful subscribe",
			setupClient: func(t *testing.T) (*Client, func()) {
	s, err := miniredis.Run()
	require.NoError(t, err)

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client.UseLogger(mockLogger)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:      testTopic,
			setupMocks: func(t *testing.T, client *Client) {
	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	client.UseMetrics(mockMetrics)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", testTopic).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_success_count", "topic", testTopic).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", testTopic).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_success_count", "topic", testTopic).AnyTimes()
			},
			publishMsg: true,
			wantErr:    false,
			validateMsg: func(t *testing.T, msg *pubsub.Message) {
	require.NotNil(t, msg)
	assert.Equal(t, testTopic, msg.Topic)
	assert.Equal(t, testMessage, msg.Value)
			},
		},
		{
			name: "no connection",
			setupClient: func(t *testing.T) (*Client, func()) {
				return New(DefaultConfig()), func() {}
			},
			topic:      testTopic,
			setupMocks: func(t *testing.T, client *Client) {},
			wantErr:    true,
			validateErr: func(t *testing.T, err error) {
				assert.Equal(t, errClientNotConnected, err)
			},
		},
		{
			name: "empty topic",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:      "",
			setupMocks: func(t *testing.T, client *Client) {},
			wantErr:    true,
			validateErr: func(t *testing.T, err error) {
				assert.Equal(t, errEmptyTopicName, err)
			},
		},
		{
			name: "context timeout",
			setupClient: func(t *testing.T) (*Client, func()) {
	s, err := miniredis.Run()
	require.NoError(t, err)

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				client.UseLogger(mockLogger)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:      testTopic,
			setupMocks: func(t *testing.T, client *Client) {
				ctrl := gomock.NewController(t)
				mockMetrics := NewMockMetrics(ctrl)
				client.UseMetrics(mockMetrics)
				mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", testTopic)
			},
			publishMsg: false,
			wantErr:    false,
			validateMsg: func(t *testing.T, msg *pubsub.Message) {
				assert.Nil(t, msg) // Should be nil due to timeout
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()
			tt.setupMocks(t, client)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if tt.publishMsg {
				go func() {
					time.Sleep(200 * time.Millisecond)
					_ = client.Publish(context.Background(), tt.topic, testMessage)
				}()
			}

			msg, err := client.Subscribe(ctx, tt.topic)

			if tt.wantErr {
	require.Error(t, err)
				if tt.validateErr != nil {
					tt.validateErr(t, err)
				}
			} else {
				if tt.validateMsg != nil {
					tt.validateMsg(t, msg)
				}
			}
		})
	}
}

func TestUnsubscribe(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		topic       string
		setupMocks  func(t *testing.T, client *Client)
		wantErr     bool
		validateErr func(t *testing.T, err error)
	}{
		{
			name: "successful unsubscribe",
			setupClient: func(t *testing.T) (*Client, func()) {
	s, err := miniredis.Run()
	require.NoError(t, err)

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client.UseLogger(mockLogger)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

				// Subscribe first
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	client.UseMetrics(mockMetrics)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", testTopic).AnyTimes()

	go func() {
		_, _ = client.Subscribe(ctx, testTopic)
	}()
	time.Sleep(200 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
					ctrl.Finish()
				}
			},
			topic:      testTopic,
			setupMocks: func(t *testing.T, client *Client) {},
			wantErr:    false,
		},
		{
			name: "unsubscribe non-existent topic",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
	require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
	time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:      "non-existent-topic",
			setupMocks: func(t *testing.T, client *Client) {},
			wantErr:    false,
		},
		{
			name: "no connection",
			setupClient: func(t *testing.T) (*Client, func()) {
				return New(DefaultConfig()), func() {}
			},
			topic:      testTopic,
			setupMocks: func(t *testing.T, client *Client) {},
			wantErr:    true,
			validateErr: func(t *testing.T, err error) {
				assert.Equal(t, errClientNotConnected, err)
			},
		},
		{
			name: "empty topic",
			setupClient: func(t *testing.T) (*Client, func()) {
	s, err := miniredis.Run()
	require.NoError(t, err)

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:      "",
			setupMocks: func(t *testing.T, client *Client) {},
			wantErr:    true,
			validateErr: func(t *testing.T, err error) {
				assert.Equal(t, errEmptyTopicName, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()
			tt.setupMocks(t, client)

			err := client.Unsubscribe(tt.topic)

			if tt.wantErr {
	require.Error(t, err)
				if tt.validateErr != nil {
					tt.validateErr(t, err)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestQuery(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		topic       string
		args        []any
		publishMsgs []string
		wantErr     bool
		validateErr func(t *testing.T, err error)
		validateRes func(t *testing.T, result []byte)
	}{
		{
			name: "successful query with messages",
			setupClient: func(t *testing.T) (*Client, func()) {
	s, err := miniredis.Run()
	require.NoError(t, err)

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:       testTopic,
			args:        []any{2 * time.Second, 2},
			publishMsgs: []string{"message1", "message2"},
			wantErr:     false,
			validateRes: func(t *testing.T, result []byte) {
				assert.NotEmpty(t, result)
				assert.Contains(t, string(result), "message1")
				assert.Contains(t, string(result), "message2")
			},
		},
		{
			name: "query with timeout",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:       testTopic,
			args:        []any{100 * time.Millisecond, 10},
			publishMsgs: nil,
			wantErr:     false,
			validateRes: func(t *testing.T, result []byte) {
				assert.Empty(t, result) // No messages published, should be empty
			},
		},
		{
			name: "no connection",
			setupClient: func(t *testing.T) (*Client, func()) {
				return New(DefaultConfig()), func() {}
			},
			topic:       testTopic,
			args:        []any{},
			publishMsgs: nil,
			wantErr:     true,
			validateErr: func(t *testing.T, err error) {
				assert.Equal(t, errClientNotConnected, err)
			},
		},
		{
			name: "empty topic",
			setupClient: func(t *testing.T) (*Client, func()) {
	s, err := miniredis.Run()
	require.NoError(t, err)

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:       "",
			args:        []any{},
			publishMsgs: nil,
			wantErr:     true,
			validateErr: func(t *testing.T, err error) {
				assert.Equal(t, errEmptyTopicName, err)
			},
		},
		{
			name: "query with limit",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:       testTopic,
			args:        []any{2 * time.Second, 1},
			publishMsgs: []string{"msg1", "msg2", "msg3"},
			wantErr:     false,
			validateRes: func(t *testing.T, result []byte) {
				// Should only get 1 message due to limit
				resultStr := string(result)
				assert.NotEmpty(t, resultStr)
				// Count newlines to verify limit
				newlineCount := strings.Count(resultStr, "\n")
				assert.Equal(t, 0, newlineCount) // Only 1 message, no newline
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()

	ctx := context.Background()

			if len(tt.publishMsgs) > 0 {
	go func() {
		time.Sleep(100 * time.Millisecond)
					for _, msg := range tt.publishMsgs {
						_ = client.Publish(ctx, tt.topic, []byte(msg))
						time.Sleep(50 * time.Millisecond)
					}
				}()
			}

			result, err := client.Query(ctx, tt.topic, tt.args...)

			if tt.wantErr {
	require.Error(t, err)
				if tt.validateErr != nil {
					tt.validateErr(t, err)
				}
			} else {
				require.NoError(t, err)
				if tt.validateRes != nil {
					tt.validateRes(t, result)
				}
			}
		})
	}
}

func TestHealth(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		validate    func(t *testing.T, health datasource.Health)
	}{
		{
			name: "health up",
			setupClient: func(t *testing.T) (*Client, func()) {
	s, err := miniredis.Run()
	require.NoError(t, err)

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			validate: func(t *testing.T, health datasource.Health) {
				assert.Equal(t, "UP", health.Status)
				assert.Equal(t, "REDIS", health.Details["backend"])
				assert.NotEmpty(t, health.Details["addr"])
			},
		},
		{
			name: "health down - no connection",
			setupClient: func(t *testing.T) (*Client, func()) {
				return New(DefaultConfig()), func() {}
			},
			validate: func(t *testing.T, health datasource.Health) {
				assert.Equal(t, "DOWN", health.Status)
				assert.Equal(t, "REDIS", health.Details["backend"])
			},
		},
		{
			name: "health down - connection failed",
			setupClient: func(t *testing.T) (*Client, func()) {
	s, err := miniredis.Run()
	require.NoError(t, err)

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	client.Connect()
	time.Sleep(100 * time.Millisecond)
				s.Close() // Close Redis to simulate failure

				return client, func() {
					_ = client.Close()
				}
			},
			validate: func(t *testing.T, health datasource.Health) {
				assert.Equal(t, "DOWN", health.Status)
	assert.Equal(t, "REDIS", health.Details["backend"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()

	health := client.Health()
			tt.validate(t, health)
		})
	}
}

func TestCreateTopic(t *testing.T) {
	tests := []struct {
		name     string
		topic    string
		wantErr  bool
		validate func(t *testing.T, err error)
	}{
		{
			name:    "create topic - no-op",
			topic:   testTopic,
			wantErr: false,
			validate: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:    "create topic with empty name",
			topic:   "",
			wantErr: false,
			validate: func(t *testing.T, err error) {
				assert.NoError(t, err) // No-op, so no error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New(DefaultConfig())
	ctx := context.Background()
			err := client.CreateTopic(ctx, tt.topic)
			tt.validate(t, err)
		})
	}
}

func TestDeleteTopic(t *testing.T) {
	tests := []struct {
		name     string
		topic    string
		wantErr  bool
		validate func(t *testing.T, err error)
	}{
		{
			name:    "delete topic - no-op",
			topic:   testTopic,
			wantErr: false,
			validate: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:    "delete topic with empty name",
			topic:   "",
			wantErr: false,
			validate: func(t *testing.T, err error) {
				assert.NoError(t, err) // No-op, so no error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New(DefaultConfig())
	ctx := context.Background()
			err := client.DeleteTopic(ctx, tt.topic)
			tt.validate(t, err)
		})
	}
}

func TestClose(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		wantErr     bool
		validate    func(t *testing.T, err error)
	}{
		{
			name: "close with connections",
			setupClient: func(t *testing.T) (*Client, func()) {
	s, err := miniredis.Run()
	require.NoError(t, err)

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

	// Subscribe to create some state
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	client.UseMetrics(mockMetrics)
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_total_count", "topic", testTopic).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_subscribe_success_count", "topic", testTopic).AnyTimes()

	go func() {
		_, _ = client.Subscribe(ctx, testTopic)
	}()
	time.Sleep(200 * time.Millisecond)

				return client, func() {
					s.Close()
					ctrl.Finish()
				}
			},
			wantErr: false,
			validate: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "close without connections",
			setupClient: func(t *testing.T) (*Client, func()) {
				return New(DefaultConfig()), func() {}
			},
			wantErr: false,
			validate: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()

			err := client.Close()
			tt.validate(t, err)
		})
	}
}


func TestRetryConnect(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		validate    func(t *testing.T, client *Client)
	}{
		{
			name: "retry connect triggers on connection failure",
			setupClient: func(t *testing.T) (*Client, func()) {
				// Start with invalid address
				cfg := &Config{
					Addr:        "invalid:6379",
					DB:          0,
					DialTimeout: 100 * time.Millisecond,
				}

				client := New(cfg)
				// Use real logger to avoid goroutine issues with mocks
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				client.UseLogger(mockLogger)

				client.Connect()

				return client, func() {
					// Close immediately to stop retry goroutine
					_ = client.Close()
				}
			},
			validate: func(t *testing.T, client *Client) {
				// Connections are created even if they fail to connect
				// The retry mechanism runs in background
				assert.NotNil(t, client.pubConn) // Client is created, connection just fails
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()
			tt.validate(t, client)
		})
	}
}

func TestTestConnections(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		wantErr     bool
		validate    func(t *testing.T, err error)
	}{
		{
			name: "test connections success",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
	require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
	time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			wantErr: false,
			validate: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "test connections - nil pubConn",
			setupClient: func(t *testing.T) (*Client, func()) {
				return New(DefaultConfig()), func() {}
			},
			wantErr: true,
			validate: func(t *testing.T, err error) {
				assert.Equal(t, errClientNotConnected, err)
			},
		},
		{
			name: "test connections - nil subConn",
			setupClient: func(t *testing.T) (*Client, func()) {
				client := New(DefaultConfig())
				// Manually set pubConn but not subConn
				s, err := miniredis.Run()
				require.NoError(t, err)
				cfg := DefaultConfig()
				cfg.Addr = s.Addr()
				options, _ := createRedisOptions(cfg)
				client.pubConn = redis.NewClient(options)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			wantErr: true,
			validate: func(t *testing.T, err error) {
				assert.Equal(t, errClientNotConnected, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()

			err := client.testConnections()
			tt.validate(t, err)
		})
	}
}

func TestSubscribeToChannel(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		topic       string
		setupMocks  func(t *testing.T, client *Client)
		validate    func(t *testing.T, client *Client)
	}{
		{
			name: "subscribe with nil subConn",
			setupClient: func(t *testing.T) (*Client, func()) {
	client := New(DefaultConfig())
				ctrl := gomock.NewController(t)
				mockLogger := NewMockLogger(ctrl)
				mockLogger.EXPECT().Errorf("subscriber connection is nil for topic '%s'", testTopic)
				client.UseLogger(mockLogger)

				return client, func() {
					ctrl.Finish()
				}
			},
			topic:      testTopic,
			setupMocks: func(t *testing.T, client *Client) {},
			validate: func(t *testing.T, client *Client) {
				ctx := context.Background()
				client.subscribeToChannel(ctx, testTopic)
				// Should return early without error
			},
		},
		{
			name: "subscribe with channel full scenario",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
	require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				client.UseLogger(mockLogger)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				// Create a full channel
				client.mu.Lock()
				client.receiveChan[testTopic] = make(chan *pubsub.Message, 1)
				client.chanClosed[testTopic] = false
				client.mu.Unlock()

				// Fill the channel
				client.receiveChan[testTopic] <- pubsub.NewMessage(context.Background())

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:      testTopic,
			setupMocks: func(t *testing.T, client *Client) {},
			validate: func(t *testing.T, client *Client) {
				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()

				// Start subscription in goroutine
				go client.subscribeToChannel(ctx, testTopic)

				// Publish a message
				time.Sleep(100 * time.Millisecond)
				_ = client.Publish(context.Background(), testTopic, testMessage)

				// Give time for processing
				time.Sleep(200 * time.Millisecond)
			},
		},
		{
			name: "subscribe with context cancellation",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				client.UseLogger(mockLogger)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:      testTopic,
			setupMocks: func(t *testing.T, client *Client) {},
			validate: func(t *testing.T, client *Client) {
				ctx, cancel := context.WithCancel(context.Background())

				// Start subscription
				done := make(chan struct{})
				go func() {
					client.subscribeToChannel(ctx, testTopic)
					close(done)
				}()

				// Cancel context
				time.Sleep(100 * time.Millisecond)
				cancel()

				// Wait for goroutine to finish
				select {
				case <-done:
					// Success
				case <-time.After(1 * time.Second):
					t.Error("subscribeToChannel did not return on context cancellation")
				}
			},
		},
		{
			name: "subscribe with nil message",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				client.UseLogger(mockLogger)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:      testTopic,
			setupMocks: func(t *testing.T, client *Client) {},
			validate: func(t *testing.T, client *Client) {
				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()

				// Start subscription
				go client.subscribeToChannel(ctx, testTopic)

				// Give time for subscription to start
				time.Sleep(100 * time.Millisecond)
				// Nil messages are skipped, so this should not cause issues
			},
		},
		{
			name: "subscribe with closed channel",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				client.UseLogger(mockLogger)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:      testTopic,
			setupMocks: func(t *testing.T, client *Client) {},
			validate: func(t *testing.T, client *Client) {
				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()

				// Mark channel as closed
				client.mu.Lock()
				client.receiveChan[testTopic] = make(chan *pubsub.Message, 10)
				client.chanClosed[testTopic] = true
				client.mu.Unlock()

				// Start subscription
				go client.subscribeToChannel(ctx, testTopic)

				// Publish message
				time.Sleep(100 * time.Millisecond)
				_ = client.Publish(context.Background(), testTopic, testMessage)

				// Give time for processing
				time.Sleep(200 * time.Millisecond)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()
			tt.setupMocks(t, client)
			tt.validate(t, client)
		})
	}
}

func TestRestartSubscriptions(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		validate    func(t *testing.T, client *Client)
	}{
		{
			name: "restart with active subscriptions and waitgroups",
			setupClient: func(t *testing.T) (*Client, func()) {
	s, err := miniredis.Run()
	require.NoError(t, err)

	cfg := DefaultConfig()
	cfg.Addr = s.Addr()

	client := New(cfg)
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client.UseLogger(mockLogger)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				// Set up complex subscription state
				client.mu.Lock()
				topics := []string{"topic1", "topic2"}
				for _, topic := range topics {
					client.subStarted[topic] = struct{}{}
					client.receiveChan[topic] = make(chan *pubsub.Message, 10)
					ctx, cancel := context.WithCancel(context.Background())
					client.subCancel[topic] = cancel
					wg := &sync.WaitGroup{}
					wg.Add(1)
					client.subWg[topic] = wg
					// Create a mock PubSub
					client.subPubSub[topic] = client.subConn.Subscribe(ctx, topic)
					// Complete the waitgroup
					wg.Done()
				}
				client.mu.Unlock()

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			validate: func(t *testing.T, client *Client) {
				client.restartSubscriptions()

				client.mu.RLock()
				assert.Equal(t, 0, len(client.subStarted))
				assert.Equal(t, 0, len(client.subCancel))
				assert.Equal(t, 0, len(client.subPubSub))
				assert.Equal(t, 0, len(client.subWg))
				client.mu.RUnlock()
			},
		},
		{
			name: "restart with timeout on waitgroup",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				client.UseLogger(mockLogger)
	client.Connect()
	time.Sleep(100 * time.Millisecond)

				// Set up subscription with waitgroup that won't complete
	client.mu.Lock()
	client.subStarted[testTopic] = struct{}{}
	client.receiveChan[testTopic] = make(chan *pubsub.Message, 10)
	_, cancel := context.WithCancel(context.Background())
	client.subCancel[testTopic] = cancel
				wg := &sync.WaitGroup{}
				wg.Add(1) // Never call Done(), will timeout
				client.subWg[testTopic] = wg
	client.mu.Unlock()

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			validate: func(t *testing.T, client *Client) {
				// This should handle timeout gracefully
	client.restartSubscriptions()

	client.mu.RLock()
				assert.Equal(t, 0, len(client.subStarted))
	client.mu.RUnlock()
			},
		},
		{
			name: "restart with subscription without cancel",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				// Set up subscription without cancel
				client.mu.Lock()
				client.subStarted[testTopic] = struct{}{}
				client.receiveChan[testTopic] = make(chan *pubsub.Message, 10)
				// No cancel set
				wg := &sync.WaitGroup{}
				wg.Add(1)
				wg.Done()
				client.subWg[testTopic] = wg
				client.mu.Unlock()

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			validate: func(t *testing.T, client *Client) {
				client.restartSubscriptions()

				client.mu.RLock()
				assert.Empty(t, client.subStarted)
				client.mu.RUnlock()
			},
		},
		{
			name: "restart with subscription without pubSub",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				// Set up subscription without pubSub
				client.mu.Lock()
				client.subStarted[testTopic] = struct{}{}
				_, cancel := context.WithCancel(context.Background())
				client.subCancel[testTopic] = cancel
				// No pubSub set
				client.mu.Unlock()

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			validate: func(t *testing.T, client *Client) {
				client.restartSubscriptions()

				client.mu.RLock()
				assert.Empty(t, client.subStarted)
				client.mu.RUnlock()
			},
		},
		{
			name: "restart with no subscriptions",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			validate: func(t *testing.T, client *Client) {
				// Call restartSubscriptions with no active subscriptions
				client.restartSubscriptions()

				// Should not panic and should clear any state
				client.mu.RLock()
				assert.Empty(t, client.subStarted)
				client.mu.RUnlock()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()
			tt.validate(t, client)
		})
	}
}


func TestPublishErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		topic       string
		message     []byte
		setupMocks  func(t *testing.T, client *Client)
		wantErr     bool
		validateErr func(t *testing.T, err error)
	}{
		{
			name: "publish with error from redis",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				client.UseLogger(mockLogger)
				client.Connect()
				time.Sleep(100 * time.Millisecond)
				s.Close() // Close Redis to cause publish error

				return client, func() {
					_ = client.Close()
				}
			},
			topic:   testTopic,
			message: testMessage,
			setupMocks: func(t *testing.T, client *Client) {
				ctrl := gomock.NewController(t)
				mockMetrics := NewMockMetrics(ctrl)
				client.UseMetrics(mockMetrics)
				mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", testTopic)
			},
			wantErr: true,
		},
		{
			name: "publish with disconnected client",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)
				_ = client.pubConn.Close() // Close connection manually

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:   testTopic,
			message: testMessage,
			setupMocks: func(t *testing.T, client *Client) {
				ctrl := gomock.NewController(t)
				mockMetrics := NewMockMetrics(ctrl)
				client.UseMetrics(mockMetrics)
				mockMetrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", testTopic)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()
			tt.setupMocks(t, client)

			ctx := context.Background()
			err := client.Publish(ctx, tt.topic, tt.message)

			if tt.wantErr {
				require.Error(t, err)
				if tt.validateErr != nil {
					tt.validateErr(t, err)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestQueryEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		topic       string
		args        []any
		wantErr     bool
		validate    func(t *testing.T, result []byte, err error)
	}{
		{
			name: "query with nil queryConn falls back to subConn",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				// Manually set queryConn to nil
				client.queryConn = nil

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:   testTopic,
			args:    []any{1 * time.Second, 1},
			wantErr: false,
			validate: func(t *testing.T, result []byte, err error) {
				// Should work with subConn fallback
				assert.NoError(t, err)
			},
		},
		{
			name: "query with nil subConn when queryConn is nil",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				// Set both to nil
				client.queryConn = nil
				client.subConn = nil

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:   testTopic,
			args:    []any{},
			wantErr: true,
			validate: func(t *testing.T, result []byte, err error) {
				assert.Equal(t, errClientNotConnected, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()

			ctx := context.Background()
			result, err := client.Query(ctx, tt.topic, tt.args...)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.validate != nil {
				tt.validate(t, result, err)
			}
		})
	}
}

func TestUnsubscribeEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		topic       string
		wantErr     bool
		validate    func(t *testing.T, err error)
	}{
		{
			name: "unsubscribe with timeout on waitgroup",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				client.UseLogger(mockLogger)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				// Set up subscription with waitgroup that won't complete quickly
				client.mu.Lock()
				client.subStarted[testTopic] = struct{}{}
				client.receiveChan[testTopic] = make(chan *pubsub.Message, 10)
				subCtx, cancel := context.WithCancel(context.Background())
				client.subCancel[testTopic] = cancel
				wg := &sync.WaitGroup{}
				wg.Add(1) // Never call Done()
				client.subWg[testTopic] = wg
				client.subPubSub[testTopic] = client.subConn.Subscribe(subCtx, testTopic)
				client.mu.Unlock()

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:   testTopic,
			wantErr: false,
			validate: func(t *testing.T, err error) {
				// Should handle timeout gracefully
				assert.NoError(t, err)
			},
		},
		{
			name: "unsubscribe with unsubscribe error",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				client.UseLogger(mockLogger)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				// Set up subscription
				client.mu.Lock()
				client.subStarted[testTopic] = struct{}{}
				client.receiveChan[testTopic] = make(chan *pubsub.Message, 10)
				ctx, cancel := context.WithCancel(context.Background())
				client.subCancel[testTopic] = cancel
				wg := &sync.WaitGroup{}
				wg.Add(1)
				wg.Done() // Complete immediately
				client.subWg[testTopic] = wg
				client.subPubSub[testTopic] = client.subConn.Subscribe(ctx, testTopic)
				client.mu.Unlock()

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:   testTopic,
			wantErr: false,
			validate: func(t *testing.T, err error) {
				// Should continue even if unsubscribe has error
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()

			err := client.Unsubscribe(tt.topic)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				if tt.validate != nil {
					tt.validate(t, err)
				}
			}
		})
	}
}

func TestHealthEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		validate    func(t *testing.T, health datasource.Health)
	}{
		{
			name: "health with nil logger",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				// Don't set logger
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			validate: func(t *testing.T, health datasource.Health) {
				assert.Equal(t, "UP", health.Status)
			},
		},
		{
			name: "health with ping error",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				client.UseLogger(mockLogger)
				client.Connect()
				time.Sleep(100 * time.Millisecond)
				s.Close() // Close Redis

				return client, func() {
					_ = client.Close()
				}
			},
			validate: func(t *testing.T, health datasource.Health) {
				assert.Equal(t, "DOWN", health.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()

			health := client.Health()
			tt.validate(t, health)
		})
	}
}

func TestCloseEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		validate    func(t *testing.T, err error)
	}{
		{
			name: "close with connection errors",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				// Close connections manually to simulate errors
				_ = client.pubConn.Close()
				_ = client.subConn.Close()
				_ = client.queryConn.Close()

				return client, func() {
					s.Close()
				}
			},
			validate: func(t *testing.T, err error) {
				// Close should handle errors gracefully
				assert.NoError(t, err)
			},
		},
		{
			name: "close with active subscriptions",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				client.UseLogger(mockLogger)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				// Set up multiple subscriptions
				client.mu.Lock()
				topics := []string{"topic1", "topic2", "topic3"}
				for _, topic := range topics {
					client.subStarted[topic] = struct{}{}
					client.receiveChan[topic] = make(chan *pubsub.Message, 10)
					ctx, cancel := context.WithCancel(context.Background())
					client.subCancel[topic] = cancel
					wg := &sync.WaitGroup{}
					wg.Add(1)
					wg.Done() // Complete immediately
					client.subWg[topic] = wg
					client.subPubSub[topic] = client.subConn.Subscribe(ctx, topic)
				}
				client.mu.Unlock()

				return client, func() {
					s.Close()
				}
			},
			validate: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()

			err := client.Close()
			tt.validate(t, err)
		})
	}
}

func TestValidateSubscribe(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		topic       string
		wantErr     bool
		validateErr func(t *testing.T, err error)
	}{
		{
			name: "nil subConn",
			setupClient: func(t *testing.T) (*Client, func()) {
				return New(DefaultConfig()), func() {}
			},
			topic:   testTopic,
			wantErr: true,
			validateErr: func(t *testing.T, err error) {
				assert.Equal(t, errClientNotConnected, err)
			},
		},
		{
			name: "not connected",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				// Close connections to simulate not connected
				_ = client.pubConn.Close()
				_ = client.subConn.Close()
				_ = client.queryConn.Close()

				// Wait a bit for isConnected to detect disconnection
				time.Sleep(200 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:   testTopic,
			wantErr: true,
			validateErr: func(t *testing.T, err error) {
				assert.Equal(t, errClientNotConnected, err)
			},
		},
		{
			name: "empty topic",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:   "",
			wantErr: true,
			validateErr: func(t *testing.T, err error) {
				assert.Equal(t, errEmptyTopicName, err)
			},
		},
		{
			name: "valid subscription",
			setupClient: func(t *testing.T) (*Client, func()) {
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			topic:   testTopic,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()

			err := client.validateSubscribe(tt.topic)

			if tt.wantErr {
				require.Error(t, err)
				if tt.validateErr != nil {
					tt.validateErr(t, err)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

