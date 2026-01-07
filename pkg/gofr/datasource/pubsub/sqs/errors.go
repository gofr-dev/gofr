package sqs

import "errors"

var (
	// ErrClientNotConnected is returned when the SQS client is not connected.
	ErrClientNotConnected = errors.New("sqs client not connected")

	// ErrQueueNotFound is returned when a queue cannot be found.
	ErrQueueNotFound = errors.New("sqs queue not found")

	// ErrEmptyQueueName is returned when an empty queue name is provided.
	ErrEmptyQueueName = errors.New("queue name cannot be empty")

	// ErrInvalidMessage is returned when message data is invalid.
	ErrInvalidMessage = errors.New("invalid message data")

	// ErrMessageReceiptHandleEmpty is returned when trying to delete/ack a message without receipt handle.
	ErrMessageReceiptHandleEmpty = errors.New("message receipt handle is empty")
)
