package sqs

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/datasource"
)

func TestClient_Health_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	health := client.Health()

	assert.Equal(t, datasource.StatusDown, health.Status)
	assert.Equal(t, "SQS", health.Details["backend"])
	assert.Equal(t, "us-east-1", health.Details["region"])
	assert.Equal(t, "client not connected", health.Details["error"])
}

func TestClient_Health_NilConfig(t *testing.T) {
	client := New(nil)
	client.UseLogger(NewMockLogger())

	health := client.Health()

	assert.Equal(t, datasource.StatusDown, health.Status)
	assert.Equal(t, "SQS", health.Details["backend"])
	assert.Empty(t, health.Details["region"])
	assert.Equal(t, "client not connected", health.Details["error"])
}

func TestClient_Health_Connected(t *testing.T) {
	mockClient := &mockSQSClient{
		listQueuesFunc: func(context.Context, *sqs.ListQueuesInput) (*sqs.ListQueuesOutput, error) {
			return &sqs.ListQueuesOutput{}, nil
		},
	}
	client := newTestClient(mockClient)

	health := client.Health()

	assert.Equal(t, datasource.StatusUp, health.Status)
	assert.Equal(t, "SQS", health.Details["backend"])
	assert.Equal(t, "us-east-1", health.Details["region"])
	assert.Nil(t, health.Details["error"])
}

func TestClient_Health_ListQueuesError(t *testing.T) {
	mockClient := &mockSQSClient{
		listQueuesFunc: func(context.Context, *sqs.ListQueuesInput) (*sqs.ListQueuesOutput, error) {
			return nil, errMockListQueues
		},
	}
	client := newTestClient(mockClient)

	health := client.Health()

	assert.Equal(t, datasource.StatusDown, health.Status)
	assert.Equal(t, "SQS", health.Details["backend"])
	assert.Equal(t, "us-east-1", health.Details["region"])
	assert.Equal(t, "list queues failed", health.Details["error"])
}
