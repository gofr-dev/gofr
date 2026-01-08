package sqs

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"gofr.dev/pkg/gofr/datasource"
)

const healthCheckTimeout = 5 * time.Second

// Health returns the health status of the SQS connection.
func (c *Client) Health() datasource.Health {
	health := datasource.Health{
		Details: map[string]any{
			"backend": "SQS",
			"region":  c.cfg.Region,
		},
	}

	if c.conn == nil {
		health.Status = datasource.StatusDown
		health.Details["error"] = "client not connected"

		return health
	}

	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()

	// Use ListQueues with MaxResults=1 as a lightweight health check
	_, err := c.conn.ListQueues(ctx, &sqs.ListQueuesInput{
		MaxResults: aws.Int32(1),
	})
	if err != nil {
		health.Status = datasource.StatusDown
		health.Details["error"] = err.Error()

		return health
	}

	health.Status = datasource.StatusUp

	return health
}
