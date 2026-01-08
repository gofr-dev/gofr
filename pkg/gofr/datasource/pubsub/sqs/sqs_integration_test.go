package sqs

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests require LocalStack to be running.
// Start LocalStack with: localstack start
// Or with Docker: docker run -d -p 4566:4566 localstack/localstack

const (
	testRegion   = "us-east-1"
	testEndpoint = "http://localhost:4566"
	testQueue    = "test-queue"
)

func getTestClient(t *testing.T) *Client {
	t.Helper()

	cfg := &Config{
		Region:          testRegion,
		Endpoint:        testEndpoint,
		AccessKeyID:     "test",
		SecretAccessKey: "test",
		WaitTimeSeconds: 1, // Short wait for tests
	}

	client := New(cfg)
	client.UseLogger(NewMockLogger())
	client.UseMetrics(NewMockMetrics())
	client.Connect()

	// Wait for connection
	time.Sleep(500 * time.Millisecond)

	return client
}

func TestIntegration_CreateAndDeleteQueue(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	ctx := context.Background()
	queueName := "integration-test-queue-" + time.Now().Format("20060102150405")

	// Create queue
	err := client.CreateTopic(ctx, queueName)
	require.NoError(t, err)

	// Delete queue
	err = client.DeleteTopic(ctx, queueName)
	require.NoError(t, err)
}

func TestIntegration_PublishAndSubscribe(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	ctx := context.Background()
	queueName := "integration-pubsub-" + time.Now().Format("20060102150405")

	// Create queue
	err := client.CreateTopic(ctx, queueName)
	require.NoError(t, err)

	defer func() {
		err = client.DeleteTopic(ctx, queueName)
		assert.NoError(t, err)
	}()

	// Publish message
	testMessage := map[string]string{
		"hello": "world",
		"test":  "message",
	}
	msgBytes, _ := json.Marshal(testMessage)

	err = client.Publish(ctx, queueName, msgBytes)
	require.NoError(t, err)

	// Subscribe and receive message
	msg, err := client.Subscribe(ctx, queueName)
	require.NoError(t, err)
	require.NotNil(t, msg)

	assert.Equal(t, queueName, msg.Topic)
	assert.Equal(t, msgBytes, msg.Value)

	// Commit the message (delete from queue)
	msg.Commit()
}

func TestIntegration_Query(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	ctx := context.Background()
	queueName := "integration-query-" + time.Now().Format("20060102150405")

	// Create queue
	err := client.CreateTopic(ctx, queueName)
	require.NoError(t, err)

	defer func() {
		err = client.DeleteTopic(ctx, queueName)
		assert.NoError(t, err)
	}()

	// Publish multiple messages
	for i := 0; i < 3; i++ {
		msg := map[string]int{"index": i}
		msgBytes, _ := json.Marshal(msg)
		err = client.Publish(ctx, queueName, msgBytes)
		require.NoError(t, err)
	}

	// Wait for messages to be available
	time.Sleep(500 * time.Millisecond)

	// Query messages
	result, err := client.Query(ctx, queueName, 3)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Result should be a JSON array
	var messages []map[string]int

	err = json.Unmarshal(result, &messages)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(messages), 1)
}

func TestIntegration_Health(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	health := client.Health()
	assert.Equal(t, "UP", health.Status)
	assert.Equal(t, "SQS", health.Details["backend"])
	assert.Equal(t, testRegion, health.Details["region"])
}
