package sqs

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// Message implements the pubsub.Committer interface for SQS messages.
type Message struct {
	receiptHandle string
	queueURL      string
	client        *sqs.Client
}

// newMessage creates a new Message for acknowledging SQS messages.
func newMessage(receiptHandle, queueURL string, client *sqs.Client) *Message {
	return &Message{
		receiptHandle: receiptHandle,
		queueURL:      queueURL,
		client:        client,
	}
}

// Commit deletes the message from the SQS queue, acknowledging its successful processing.
func (m *Message) Commit() {
	if m.receiptHandle == "" || m.client == nil {
		return
	}

	_, _ = m.client.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{
		QueueUrl:      &m.queueURL,
		ReceiptHandle: &m.receiptHandle,
	})
}
