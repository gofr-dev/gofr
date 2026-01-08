package sqs

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// messageClient interface for message deletion operations.
type messageClient interface {
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

// Message implements the pubsub.Committer interface for SQS messages.
type Message struct {
	receiptHandle string
	queueURL      string
	client        messageClient
	logger        pubsub.Logger
}

// newMessage creates a new Message for acknowledging SQS messages.
func newMessage(receiptHandle, queueURL string, client messageClient, logger pubsub.Logger) *Message {
	return &Message{
		receiptHandle: receiptHandle,
		queueURL:      queueURL,
		client:        client,
		logger:        logger,
	}
}

// Commit deletes the message from the SQS queue, acknowledging its successful processing.
func (m *Message) Commit() {
	if m.receiptHandle == "" || m.client == nil {
		return
	}

	_, err := m.client.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{
		QueueUrl:      &m.queueURL,
		ReceiptHandle: &m.receiptHandle,
	})
	if err != nil && m.logger != nil {
		m.logger.Errorf("failed to delete SQS message: %v", err)
	}
}
