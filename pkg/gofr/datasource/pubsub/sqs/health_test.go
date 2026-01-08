package sqs

import (
	"testing"

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
	assert.Equal(t, "client not connected", health.Details["error"])
}
