package sqs

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
)

// mockMessageClient is a mock for messageClient interface.
type mockMessageClient struct {
	deleteMessageFunc func(ctx context.Context,
		params *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error)
}

func (m *mockMessageClient) DeleteMessage(
	ctx context.Context, params *sqs.DeleteMessageInput, _ ...func(*sqs.Options),
) (*sqs.DeleteMessageOutput, error) {
	if m.deleteMessageFunc != nil {
		return m.deleteMessageFunc(ctx, params)
	}

	return &sqs.DeleteMessageOutput{}, nil
}

func TestNewMessage(t *testing.T) {
	logger := NewMockLogger()
	mockClient := &mockMessageClient{}
	msg := newMessage("receipt-123", "http://localhost:4566/queue/test", mockClient, logger)

	assert.NotNil(t, msg)
	assert.Equal(t, "receipt-123", msg.receiptHandle)
	assert.Equal(t, "http://localhost:4566/queue/test", msg.queueURL)
	assert.NotNil(t, msg.client)
	assert.Equal(t, logger, msg.logger)
}

func TestMessage_Commit_EmptyReceiptHandle(*testing.T) {
	msg := &Message{
		receiptHandle: "",
		queueURL:      "http://localhost:4566/queue/test",
		client:        &mockMessageClient{},
		logger:        NewMockLogger(),
	}

	// Should not panic, just return early
	msg.Commit()
}

func TestMessage_Commit_NilClient(*testing.T) {
	msg := &Message{
		receiptHandle: "receipt-123",
		queueURL:      "http://localhost:4566/queue/test",
		client:        nil,
		logger:        NewMockLogger(),
	}

	// Should not panic, just return early
	msg.Commit()
}

func TestMessage_Commit_Success(t *testing.T) {
	deleteMessageCalled := false
	mockClient := &mockMessageClient{
		deleteMessageFunc: func(context.Context, *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error) {
			deleteMessageCalled = true

			return &sqs.DeleteMessageOutput{}, nil
		},
	}

	msg := &Message{
		receiptHandle: "receipt-123",
		queueURL:      "http://localhost:4566/queue/test",
		client:        mockClient,
		logger:        NewMockLogger(),
	}

	msg.Commit()
	assert.True(t, deleteMessageCalled)
}

func TestMessage_Commit_Error(t *testing.T) {
	mockClient := &mockMessageClient{
		deleteMessageFunc: func(context.Context, *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error) {
			return nil, errMockDeleteFailed
		},
	}

	logger := NewMockLogger()
	msg := &Message{
		receiptHandle: "receipt-123",
		queueURL:      "http://localhost:4566/queue/test",
		client:        mockClient,
		logger:        logger,
	}

	msg.Commit()
	// Should log error but not panic
	assert.NotEmpty(t, logger.lastError)
}

func TestMessage_Commit_ErrorWithNilLogger(*testing.T) {
	mockClient := &mockMessageClient{
		deleteMessageFunc: func(context.Context, *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error) {
			return nil, errMockDeleteFailed
		},
	}

	msg := &Message{
		receiptHandle: "receipt-123",
		queueURL:      "http://localhost:4566/queue/test",
		client:        mockClient,
		logger:        nil,
	}

	// Should not panic even with nil logger
	msg.Commit()
}

func TestMessage_Commit_BothEmpty(*testing.T) {
	msg := &Message{
		receiptHandle: "",
		queueURL:      "",
		client:        nil,
		logger:        nil,
	}

	// Should not panic
	msg.Commit()
}
