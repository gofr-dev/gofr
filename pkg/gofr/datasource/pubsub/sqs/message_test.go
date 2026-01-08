package sqs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMessage(t *testing.T) {
	msg := newMessage("receipt-123", "http://localhost:4566/queue/test", nil)

	assert.NotNil(t, msg)
	assert.Equal(t, "receipt-123", msg.receiptHandle)
	assert.Equal(t, "http://localhost:4566/queue/test", msg.queueURL)
	assert.Nil(t, msg.client)
}

func TestMessage_Commit_EmptyReceiptHandle(*testing.T) {
	msg := &Message{
		receiptHandle: "",
		queueURL:      "http://localhost:4566/queue/test",
		client:        nil,
	}

	// Should not panic, just return early
	msg.Commit()
}
