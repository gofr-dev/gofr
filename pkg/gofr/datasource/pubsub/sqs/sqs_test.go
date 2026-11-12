package sqs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Test errors - static errors for testing purposes.
var (
	errWrappedContextCanceled  = errors.New("operation failed: context canceled")
	errWrappedDeadlineExceeded = errors.New("timeout: context deadline exceeded")
	errRequestCanceled         = errors.New("request was canceled")
	errGeneric                 = errors.New("some other error")
	errConnectionRefused       = errors.New("dial tcp: connection refused")
	errNoSuchHost              = errors.New("dial tcp: no such host")
	errNetworkUnreachable      = errors.New("dial tcp: network is unreachable")
	errMaxAttemptsExceeded     = errors.New("exceeded maximum number of attempts")
	errMockSendMessage         = errors.New("mock send message error")
	errMockReceiveMessage      = errors.New("mock receive message error")
	errMockCreateQueue         = errors.New("mock create queue error")
	errMockDeleteQueue         = errors.New("mock delete queue error")
	errMockGetQueueURL         = errors.New("mock get queue url error")
	errMockDeleteFailed        = errors.New("delete failed")
	errMockListQueues          = errors.New("list queues failed")
)

func TestNew(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		client := New(nil)
		require.NotNil(t, client)
		assert.NotNil(t, client.cfg)
		assert.NotNil(t, client.queueURLCache)
	})

	t.Run("with config", func(t *testing.T) {
		cfg := &Config{
			Region:          "us-east-1",
			Endpoint:        "http://localhost:4566",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
			SessionToken:    "test-token",
		}
		client := New(cfg)
		require.NotNil(t, client)
		assert.Equal(t, "us-east-1", client.cfg.Region)
		assert.Equal(t, "http://localhost:4566", client.cfg.Endpoint)
		assert.Equal(t, "test-key", client.cfg.AccessKeyID)
		assert.Equal(t, "test-secret", client.cfg.SecretAccessKey)
		assert.Equal(t, "test-token", client.cfg.SessionToken)
	})
}

func TestClient_UseLogger(t *testing.T) {
	client := New(&Config{})
	logger := NewMockLogger()

	client.UseLogger(logger)
	assert.NotNil(t, client.logger)

	client.UseLogger("invalid")
	assert.Equal(t, logger, client.logger)

	client.UseLogger(nil)
	assert.Equal(t, logger, client.logger)
}

func TestClient_UseMetrics(t *testing.T) {
	client := New(&Config{})
	metrics := NewMockMetrics()

	client.UseMetrics(metrics)
	assert.NotNil(t, client.metrics)

	client.UseMetrics("invalid")
	assert.Equal(t, metrics, client.metrics)

	client.UseMetrics(nil)
	assert.Equal(t, metrics, client.metrics)
}

func TestClient_UseTracer(t *testing.T) {
	client := New(&Config{})

	client.UseTracer("invalid")
	assert.Nil(t, client.tracer)

	client.UseTracer(nil)
	assert.Nil(t, client.tracer)

	// Test with valid tracer
	tracer := otel.GetTracerProvider().Tracer("test")
	client.UseTracer(tracer)
	assert.NotNil(t, client.tracer)
}

func TestClient_Connect_NoRegion(t *testing.T) {
	client := New(&Config{})
	client.UseLogger(NewMockLogger())

	client.Connect()

	assert.Nil(t, client.conn)
}

func TestClient_Connect_NoLogger(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})

	client.Connect()

	assert.Nil(t, client.conn)
}

func TestClient_isConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	assert.False(t, client.isConnected())

	client.conn = &mockSQSClient{}
	assert.True(t, client.isConnected())
}

func TestClient_Publish_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())
	client.UseMetrics(NewMockMetrics())

	err := client.Publish(context.Background(), "test-queue", []byte("test message"))
	assert.ErrorIs(t, err, errClientNotConnected)
}

func TestClient_Publish_EmptyTopic(t *testing.T) {
	client := newTestClient(&mockSQSClient{})

	err := client.Publish(context.Background(), "", []byte("test message"))
	assert.ErrorIs(t, err, errEmptyQueueName)
}

func TestClient_Publish_Success(t *testing.T) {
	mockClient := &mockSQSClient{}
	client := newTestClient(mockClient)

	err := client.Publish(context.Background(), "test-queue", []byte(`{"test":"data"}`))
	assert.NoError(t, err)
}

func TestClient_Publish_GetQueueURLError(t *testing.T) {
	mockClient := &mockSQSClient{
		getQueueURLFunc: func(context.Context, *sqs.GetQueueUrlInput) (*sqs.GetQueueUrlOutput, error) {
			return nil, errMockGetQueueURL
		},
	}
	client := newTestClient(mockClient)

	err := client.Publish(context.Background(), "test-queue", []byte("test"))
	assert.ErrorIs(t, err, errQueueNotFound)
}

func TestClient_Publish_SendMessageError(t *testing.T) {
	mockClient := &mockSQSClient{
		sendMessageFunc: func(context.Context, *sqs.SendMessageInput) (*sqs.SendMessageOutput, error) {
			return nil, errMockSendMessage
		},
	}
	client := newTestClient(mockClient)

	err := client.Publish(context.Background(), "test-queue", []byte("test"))
	assert.Error(t, err)
}

func TestClient_Subscribe_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())
	client.UseMetrics(NewMockMetrics())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg, err := client.Subscribe(ctx, "test-queue")
	require.ErrorIs(t, err, errClientNotConnected)
	assert.Nil(t, msg)
}

func TestClient_Subscribe_EmptyTopic(t *testing.T) {
	client := newTestClient(&mockSQSClient{})

	msg, err := client.Subscribe(context.Background(), "")
	require.ErrorIs(t, err, errEmptyQueueName)
	assert.Nil(t, msg)
}

func TestClient_Subscribe_Success(t *testing.T) {
	mockClient := &mockSQSClient{}
	client := newTestClient(mockClient)

	msg, err := client.Subscribe(context.Background(), "test-queue")
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "test-queue", msg.Topic)
	assert.JSONEq(t, `{"test":"data"}`, string(msg.Value))
}

func TestClient_Subscribe_NoMessages(t *testing.T) {
	mockClient := &mockSQSClient{
		receiveMessageFunc: func(context.Context, *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
			return &sqs.ReceiveMessageOutput{Messages: []types.Message{}}, nil
		},
	}
	client := newTestClient(mockClient)

	msg, err := client.Subscribe(context.Background(), "test-queue")
	require.NoError(t, err)
	assert.Nil(t, msg)
}

func TestClient_Subscribe_GetQueueURLError(t *testing.T) {
	mockClient := &mockSQSClient{
		getQueueURLFunc: func(context.Context, *sqs.GetQueueUrlInput) (*sqs.GetQueueUrlOutput, error) {
			return nil, errMockGetQueueURL
		},
	}
	client := newTestClient(mockClient)

	msg, err := client.Subscribe(context.Background(), "test-queue")
	require.ErrorIs(t, err, errQueueNotFound)
	assert.Nil(t, msg)
}

func TestClient_Subscribe_ReceiveMessageError(t *testing.T) {
	mockClient := &mockSQSClient{
		receiveMessageFunc: func(context.Context, *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
			return nil, errMockReceiveMessage
		},
	}
	client := newTestClient(mockClient)

	msg, err := client.Subscribe(context.Background(), "test-queue")
	require.Error(t, err)
	assert.Nil(t, msg)
}

func TestClient_Subscribe_ContextCanceled(t *testing.T) {
	mockClient := &mockSQSClient{
		receiveMessageFunc: func(context.Context, *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
			return nil, context.Canceled
		},
	}
	client := newTestClient(mockClient)

	msg, err := client.Subscribe(context.Background(), "test-queue")
	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, msg)
}

func TestClient_CreateTopic_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	err := client.CreateTopic(context.Background(), "test-queue")
	assert.ErrorIs(t, err, errClientNotConnected)
}

func TestClient_CreateTopic_EmptyName(t *testing.T) {
	client := newTestClient(&mockSQSClient{})

	err := client.CreateTopic(context.Background(), "")
	assert.ErrorIs(t, err, errEmptyQueueName)
}

func TestClient_CreateTopic_Success(t *testing.T) {
	client := newTestClient(&mockSQSClient{})

	err := client.CreateTopic(context.Background(), "test-queue")
	assert.NoError(t, err)
}

func TestClient_CreateTopic_Error(t *testing.T) {
	mockClient := &mockSQSClient{
		createQueueFunc: func(context.Context, *sqs.CreateQueueInput) (*sqs.CreateQueueOutput, error) {
			return nil, errMockCreateQueue
		},
	}
	client := newTestClient(mockClient)

	err := client.CreateTopic(context.Background(), "test-queue")
	assert.Error(t, err)
}

func TestClient_DeleteTopic_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	err := client.DeleteTopic(context.Background(), "test-queue")
	assert.ErrorIs(t, err, errClientNotConnected)
}

func TestClient_DeleteTopic_EmptyName(t *testing.T) {
	client := newTestClient(&mockSQSClient{})

	err := client.DeleteTopic(context.Background(), "")
	assert.ErrorIs(t, err, errEmptyQueueName)
}

func TestClient_DeleteTopic_Success(t *testing.T) {
	client := newTestClient(&mockSQSClient{})

	err := client.DeleteTopic(context.Background(), "test-queue")
	assert.NoError(t, err)
}

func TestClient_DeleteTopic_GetQueueURLError(t *testing.T) {
	mockClient := &mockSQSClient{
		getQueueURLFunc: func(context.Context, *sqs.GetQueueUrlInput) (*sqs.GetQueueUrlOutput, error) {
			return nil, errMockGetQueueURL
		},
	}
	client := newTestClient(mockClient)

	err := client.DeleteTopic(context.Background(), "test-queue")
	assert.ErrorIs(t, err, errQueueNotFound)
}

func TestClient_DeleteTopic_DeleteQueueError(t *testing.T) {
	mockClient := &mockSQSClient{
		deleteQueueFunc: func(context.Context, *sqs.DeleteQueueInput) (*sqs.DeleteQueueOutput, error) {
			return nil, errMockDeleteQueue
		},
	}
	client := newTestClient(mockClient)

	err := client.DeleteTopic(context.Background(), "test-queue")
	assert.Error(t, err)
}

func TestClient_Query_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	result, err := client.Query(context.Background(), "test-queue")
	require.ErrorIs(t, err, errClientNotConnected)
	assert.Nil(t, result)
}

func TestClient_Query_EmptyQuery(t *testing.T) {
	client := newTestClient(&mockSQSClient{})

	result, err := client.Query(context.Background(), "")
	require.ErrorIs(t, err, errEmptyQueueName)
	assert.Nil(t, result)
}

func TestClient_Query_Success(t *testing.T) {
	mockClient := &mockSQSClient{
		receiveMessageFunc: func(context.Context, *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
			return &sqs.ReceiveMessageOutput{
				Messages: []types.Message{
					{Body: aws.String(`{"id":1}`)},
					{Body: aws.String(`{"id":2}`)},
				},
			}, nil
		},
	}
	client := newTestClient(mockClient)

	result, err := client.Query(context.Background(), "test-queue", int32(5))
	require.NoError(t, err)
	assert.Equal(t, `[{"id":1},{"id":2}]`, string(result))
}

func TestClient_Query_NoMessages(t *testing.T) {
	mockClient := &mockSQSClient{
		receiveMessageFunc: func(context.Context, *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
			return &sqs.ReceiveMessageOutput{Messages: []types.Message{}}, nil
		},
	}
	client := newTestClient(mockClient)

	result, err := client.Query(context.Background(), "test-queue")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestClient_Query_GetQueueURLError(t *testing.T) {
	mockClient := &mockSQSClient{
		getQueueURLFunc: func(context.Context, *sqs.GetQueueUrlInput) (*sqs.GetQueueUrlOutput, error) {
			return nil, errMockGetQueueURL
		},
	}
	client := newTestClient(mockClient)

	result, err := client.Query(context.Background(), "test-queue")
	require.ErrorIs(t, err, errQueueNotFound)
	assert.Nil(t, result)
}

func TestClient_Query_ReceiveMessageError(t *testing.T) {
	mockClient := &mockSQSClient{
		receiveMessageFunc: func(context.Context, *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
			return nil, errMockReceiveMessage
		},
	}
	client := newTestClient(mockClient)

	result, err := client.Query(context.Background(), "test-queue")
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestClient_Close(t *testing.T) {
	client := newTestClient(&mockSQSClient{})

	err := client.Close()
	require.NoError(t, err)
	assert.False(t, client.isConnected())
}

func TestClient_getQueueURL_Cached(t *testing.T) {
	mockClient := &mockSQSClient{}
	client := newTestClient(mockClient)

	// First call - should call GetQueueUrl
	url1, err := client.getQueueURL(context.Background(), "test-queue")
	require.NoError(t, err)
	assert.Contains(t, url1, "test-queue")

	// Second call - should use cache
	url2, err := client.getQueueURL(context.Background(), "test-queue")
	require.NoError(t, err)
	assert.Equal(t, url1, url2)
}

func TestParseQueryArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []any
		expected int32
	}{
		{"no args", nil, defaultQueryMaxMessages},
		{"empty args", []any{}, defaultQueryMaxMessages},
		{"valid int32 limit 1", []any{int32(1)}, 1},
		{"valid int32 limit 5", []any{int32(5)}, 5},
		{"valid int32 limit 10", []any{int32(10)}, 10},
		{"int32 limit exceeds max", []any{int32(15)}, defaultQueryMaxMessages},
		{"invalid type int", []any{5}, defaultQueryMaxMessages},
		{"invalid type string", []any{"invalid"}, defaultQueryMaxMessages},
		{"int32 zero", []any{int32(0)}, defaultQueryMaxMessages},
		{"int32 negative", []any{int32(-1)}, defaultQueryMaxMessages},
		{"multiple args uses first", []any{int32(5), int32(3)}, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseQueryArgs(tt.args...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsContextCanceled(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"context canceled", context.Canceled, true},
		{"context deadline exceeded", context.DeadlineExceeded, true},
		{"wrapped context canceled", errWrappedContextCanceled, true},
		{"wrapped deadline exceeded", errWrappedDeadlineExceeded, true},
		{"canceled keyword", errRequestCanceled, true},
		{"other error", errClientNotConnected, false},
		{"generic error", errGeneric, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isContextCanceled(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"connection refused", errConnectionRefused, true},
		{"no such host", errNoSuchHost, true},
		{"network unreachable", errNetworkUnreachable, true},
		{"max attempts exceeded", errMaxAttemptsExceeded, true},
		{"queue not found", errQueueNotFound, false},
		{"client not connected", errClientNotConnected, false},
		{"generic error", errGeneric, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isConnectionError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStartPublishSpan_CreatesValidSpan(t *testing.T) {
	ctx, span, attrs := startPublishSpan(context.Background(), "test-queue")
	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
	assert.Implements(t, (*trace.Span)(nil), span)
	assert.NotNil(t, attrs)

	span.End()
}

func TestErrors(t *testing.T) {
	assert.Equal(t, "sqs client not connected", errClientNotConnected.Error())
	assert.Equal(t, "sqs queue not found", errQueueNotFound.Error())
	assert.Equal(t, "queue name cannot be empty", errEmptyQueueName.Error())
}

// isolateAWSEnv keeps LoadDefaultConfig away from the developer's real AWS setup: shared config and
// credentials files point at paths that do not exist, and every credential/profile env var is
// cleared. Rows that need a profile or env credentials set them on top of this.
func isolateAWSEnv(t *testing.T) {
	t.Helper()

	dir := t.TempDir()

	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_DEFAULT_PROFILE", "")
	t.Setenv("AWS_ROLE_ARN", "")
	t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "")
	t.Setenv("AWS_CONTAINER_CREDENTIALS_FULL_URI", "")
	t.Setenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
}

// newListQueuesServer fakes the SQS JSON endpoint for ListQueues, answering every request with the
// given status. It counts the ListQueues calls it receives.
func newListQueuesServer(t *testing.T, status int) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Amz-Target") == "AmazonSQS.ListQueues" {
			calls.Add(1)
		}

		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		w.WriteHeader(status)

		// A 400 with a non-throttling error code is not retried by the SDK, so the failure is immediate.
		body := map[int]string{
			http.StatusOK:         `{"QueueUrls":[]}`,
			http.StatusBadRequest: `{"__type":"com.amazonaws.sqs#AccessDenied","message":"access denied"}`,
		}[status]

		_, _ = w.Write([]byte(body))
	}))

	t.Cleanup(server.Close)

	return server, &calls
}

func TestClient_Connect(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		profile       string
		expConnected  bool
		expLastError  string
		expListQueues int32
	}{
		{
			name:          "connects when ListQueues succeeds",
			status:        http.StatusOK,
			expConnected:  true,
			expLastError:  "",
			expListQueues: 1,
		},
		{
			name:          "drops the connection when ListQueues fails",
			status:        http.StatusBadRequest,
			expConnected:  false,
			expLastError:  "failed to connect to SQS: %v",
			expListQueues: 1,
		},
		{
			name:          "gives up before dialing when the AWS config cannot load",
			status:        http.StatusOK,
			profile:       "profile-that-does-not-exist",
			expConnected:  false,
			expLastError:  "failed to load AWS config: %v",
			expListQueues: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateAWSEnv(t)
			t.Setenv("AWS_PROFILE", tt.profile)

			server, calls := newListQueuesServer(t, tt.status)

			logger := NewMockLogger()
			client := New(&Config{
				Region:          "us-east-1",
				Endpoint:        server.URL,
				AccessKeyID:     "test-key",
				SecretAccessKey: "test-secret",
			})
			client.UseLogger(logger)

			// Mark a retry as already in flight so a failed connect does not start the background
			// retry loop, which sleeps between attempts and would outlive the test.
			client.isRetrying.Store(true)

			client.Connect()

			assert.Equal(t, tt.expConnected, client.isConnected())
			assert.Equal(t, tt.expLastError, logger.lastError)
			assert.Equal(t, tt.expListQueues, calls.Load())
			assert.True(t, client.isRetrying.Load(), "Connect must not reset an in-flight retry")
		})
	}
}

func TestClient_connectInternal(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		profile       string
		expConnected  bool
		expListQueues int32
	}{
		{name: "connects when ListQueues succeeds", status: http.StatusOK, expConnected: true, expListQueues: 1},
		{name: "stays disconnected when ListQueues fails", status: http.StatusBadRequest, expConnected: false, expListQueues: 1},
		{
			name:          "stays disconnected when the AWS config cannot load",
			status:        http.StatusOK,
			profile:       "profile-that-does-not-exist",
			expConnected:  false,
			expListQueues: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateAWSEnv(t)
			t.Setenv("AWS_PROFILE", tt.profile)

			server, calls := newListQueuesServer(t, tt.status)

			client := New(&Config{
				Region:          "us-east-1",
				Endpoint:        server.URL,
				AccessKeyID:     "test-key",
				SecretAccessKey: "test-secret",
			})
			client.UseLogger(NewMockLogger())

			client.connectInternal()

			assert.Equal(t, tt.expConnected, client.isConnected())
			assert.Equal(t, tt.expListQueues, calls.Load())
		})
	}
}

func TestClient_loadAWSConfig(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *Config
		expAccessKey string
		expToken     string
	}{
		{
			name:         "static credentials take precedence",
			cfg:          &Config{Region: "eu-west-1", AccessKeyID: "static-key", SecretAccessKey: "static-secret", SessionToken: "tok"},
			expAccessKey: "static-key",
			expToken:     "tok",
		},
		{
			name:         "partial static credentials fall back to the default chain",
			cfg:          &Config{Region: "eu-west-1", AccessKeyID: "static-key"},
			expAccessKey: "env-key",
			expToken:     "",
		},
		{
			name:         "no static credentials use the default chain",
			cfg:          &Config{Region: "eu-west-1"},
			expAccessKey: "env-key",
			expToken:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateAWSEnv(t)
			t.Setenv("AWS_ACCESS_KEY_ID", "env-key")
			t.Setenv("AWS_SECRET_ACCESS_KEY", "env-secret")

			client := New(tt.cfg)

			awsCfg, err := client.loadAWSConfig(t.Context())
			require.NoError(t, err)

			creds, err := awsCfg.Credentials.Retrieve(t.Context())
			require.NoError(t, err)

			assert.Equal(t, "eu-west-1", awsCfg.Region)
			assert.Equal(t, tt.expAccessKey, creds.AccessKeyID)
			assert.Equal(t, tt.expToken, creds.SessionToken)
		})
	}
}
