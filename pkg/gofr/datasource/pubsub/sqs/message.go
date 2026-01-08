package sqs

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// Message represents an SQS message that implements the Committer interface.
type Message struct {
	receiptHandle string
	queueURL string
	messageID string
	client *sqs.Client
	logger Logger
}

// newMessage creates a new Message with the required fields for committing.
func newMessage(receiptHandle, queueURL, messageID string, client *sqs.Client, logger Logger) *Message {
	return &Message{
		receiptHandle: receiptHandle,
		queueURL:      queueURL,
		messageID:     messageID,
		client:        client,
		logger:        logger,
	}
}

// Commit deletes the message from the SQS queue, acknowledging its successful processing.
// This implements the pubsub.Committer interface.
func (m *Message) Commit() {
	if m.receiptHandle == "" {
		m.logger.Error("cannot commit message: receipt handle is empty")
		return
	}

	if m.client == nil {
		m.logger.Error("cannot commit message: SQS client is nil")
		return
	}

	_, err := m.client.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{
		QueueUrl:      &m.queueURL,
		ReceiptHandle: &m.receiptHandle,
	})
	if err != nil {
		m.logger.Errorf("failed to commit (delete) message %s: %v", m.messageID, err)
		return
	}

	m.logger.Debugf("message committed (deleted) successfully: %s", m.messageID)
}
