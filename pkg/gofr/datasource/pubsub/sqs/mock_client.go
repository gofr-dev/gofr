package sqs

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// MockLogger is a mock implementation of pubsub.Logger interface.
type MockLogger struct {
	lastError string
}

// NewMockLogger creates a new MockLogger.
func NewMockLogger() *MockLogger {
	return &MockLogger{}
}

func (*MockLogger) Debug(...any)          {}
func (*MockLogger) Debugf(string, ...any) {}
func (*MockLogger) Log(...any)            {}
func (*MockLogger) Logf(string, ...any)   {}
func (*MockLogger) Error(...any)          {}

func (m *MockLogger) Errorf(format string, args ...any) {
	if len(args) > 0 {
		m.lastError = format
	}
}

// MockMetrics is a mock implementation of Metrics interface.
type MockMetrics struct{}

// NewMockMetrics creates a new MockMetrics.
func NewMockMetrics() *MockMetrics {
	return &MockMetrics{}
}

func (*MockMetrics) IncrementCounter(context.Context, string, ...string) {}

// mockSQSClient is a mock implementation of sqsClient interface for testing.
type mockSQSClient struct {
	sendMessageFunc    func(ctx context.Context, params *sqs.SendMessageInput) (*sqs.SendMessageOutput, error)
	receiveMessageFunc func(ctx context.Context, params *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error)
	deleteMessageFunc  func(ctx context.Context, params *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error)
	createQueueFunc    func(ctx context.Context, params *sqs.CreateQueueInput) (*sqs.CreateQueueOutput, error)
	deleteQueueFunc    func(ctx context.Context, params *sqs.DeleteQueueInput) (*sqs.DeleteQueueOutput, error)
	getQueueURLFunc    func(ctx context.Context, params *sqs.GetQueueUrlInput) (*sqs.GetQueueUrlOutput, error)
	listQueuesFunc     func(ctx context.Context, params *sqs.ListQueuesInput) (*sqs.ListQueuesOutput, error)
}

func (m *mockSQSClient) SendMessage(
	ctx context.Context, params *sqs.SendMessageInput, _ ...func(*sqs.Options),
) (*sqs.SendMessageOutput, error) {
	if m.sendMessageFunc != nil {
		return m.sendMessageFunc(ctx, params)
	}

	return &sqs.SendMessageOutput{MessageId: aws.String("test-message-id")}, nil
}

func (m *mockSQSClient) ReceiveMessage(
	ctx context.Context, params *sqs.ReceiveMessageInput, _ ...func(*sqs.Options),
) (*sqs.ReceiveMessageOutput, error) {
	if m.receiveMessageFunc != nil {
		return m.receiveMessageFunc(ctx, params)
	}

	return &sqs.ReceiveMessageOutput{
		Messages: []types.Message{
			{
				MessageId:     aws.String("msg-123"),
				Body:          aws.String(`{"test":"data"}`),
				ReceiptHandle: aws.String("receipt-handle-123"),
			},
		},
	}, nil
}

func (m *mockSQSClient) DeleteMessage(
	ctx context.Context, params *sqs.DeleteMessageInput, _ ...func(*sqs.Options),
) (*sqs.DeleteMessageOutput, error) {
	if m.deleteMessageFunc != nil {
		return m.deleteMessageFunc(ctx, params)
	}

	return &sqs.DeleteMessageOutput{}, nil
}

func (m *mockSQSClient) CreateQueue(
	ctx context.Context, params *sqs.CreateQueueInput, _ ...func(*sqs.Options),
) (*sqs.CreateQueueOutput, error) {
	if m.createQueueFunc != nil {
		return m.createQueueFunc(ctx, params)
	}

	return &sqs.CreateQueueOutput{QueueUrl: aws.String("http://localhost:4566/queue/test")}, nil
}

func (m *mockSQSClient) DeleteQueue(
	ctx context.Context, params *sqs.DeleteQueueInput, _ ...func(*sqs.Options),
) (*sqs.DeleteQueueOutput, error) {
	if m.deleteQueueFunc != nil {
		return m.deleteQueueFunc(ctx, params)
	}

	return &sqs.DeleteQueueOutput{}, nil
}

//nolint:revive // GetQueueUrl matches AWS SDK method name
func (m *mockSQSClient) GetQueueUrl(
	ctx context.Context, params *sqs.GetQueueUrlInput, _ ...func(*sqs.Options),
) (*sqs.GetQueueUrlOutput, error) {
	if m.getQueueURLFunc != nil {
		return m.getQueueURLFunc(ctx, params)
	}

	return &sqs.GetQueueUrlOutput{
		QueueUrl: aws.String("http://localhost:4566/queue/" + *params.QueueName),
	}, nil
}

func (m *mockSQSClient) ListQueues(
	ctx context.Context, params *sqs.ListQueuesInput, _ ...func(*sqs.Options),
) (*sqs.ListQueuesOutput, error) {
	if m.listQueuesFunc != nil {
		return m.listQueuesFunc(ctx, params)
	}

	return &sqs.ListQueuesOutput{}, nil
}

// Helper to create a connected client with mock for testing.
func newTestClient(mockClient *mockSQSClient) *Client {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())
	client.UseMetrics(NewMockMetrics())
	client.conn = mockClient

	return client
}

// Test errors - static errors for testing purposes.
var (
	errMockSendMessage    = errors.New("mock send message error")
	errMockReceiveMessage = errors.New("mock receive message error")
	errMockCreateQueue    = errors.New("mock create queue error")
	errMockDeleteQueue    = errors.New("mock delete queue error")
	errMockGetQueueURL    = errors.New("mock get queue url error")
	errMockDeleteFailed   = errors.New("delete failed")
	errMockListQueues     = errors.New("list queues failed")
)
