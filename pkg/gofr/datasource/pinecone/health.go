package pinecone

import (
	"context"
	"fmt"

	"github.com/pinecone-io/go-pinecone/v3/pinecone"
)

// healthChecker handles health check operations.
type healthChecker struct {
	client *Client
}

// newHealthChecker creates a new health checker.
func newHealthChecker(client *Client) *healthChecker {
	return &healthChecker{client: client}
}

// check performs a health check on the Pinecone connection.
func (hc *healthChecker) check(ctx context.Context) (any, error) {
	health := hc.createHealthResponse()

	if !hc.isClientConnected() {
		return hc.handleDisconnectedHealth(health)
	}

	return hc.performConnectivityTest(ctx, health)
}

// createHealthResponse creates the initial health response structure.
func (*healthChecker) createHealthResponse() Health {
	return Health{
		Status:  statusDown,
		Details: make(map[string]any),
	}
}

// isClientConnected checks if the client is connected.
func (hc *healthChecker) isClientConnected() bool {
	return hc.client.connected && hc.client.client != nil
}

// handleDisconnectedHealth handles the case when client is not connected.
func (*healthChecker) handleDisconnectedHealth(health Health) (any, error) {
	health.Status = statusDown
	health.Details["error"] = "pinecone client not connected"
	health.Details["connection_state"] = "disconnected"

	return health, ErrClientNotConnected
}

// performConnectivityTest tests the connection by listing indexes.
func (hc *healthChecker) performConnectivityTest(ctx context.Context, health Health) (any, error) {
	indexes, err := hc.client.client.ListIndexes(ctx)
	if err != nil {
		return hc.handleConnectivityError(health, err)
	}

	return hc.buildHealthyResponse(health, indexes), nil
}

// handleConnectivityError handles connectivity test errors.
func (*healthChecker) handleConnectivityError(health Health, err error) (any, error) {
	health.Status = statusDown
	health.Details["error"] = fmt.Sprintf("failed to connect to Pinecone API: %v", err)
	health.Details["connection_state"] = "error"

	return health, err
}

// buildHealthyResponse builds a healthy response with index information.
func (hc *healthChecker) buildHealthyResponse(health Health, indexes []*pinecone.Index) Health {
	health.Status = statusUp
	health.Details["index_count"] = len(indexes)
	health.Details["connection_state"] = "connected"

	if len(indexes) > 0 {
		hc.addIndexDetails(&health, indexes)
	}

	return health
}

// addIndexDetails adds index information to the health response.
func (*healthChecker) addIndexDetails(health *Health, indexes []*pinecone.Index) {
	for i, index := range indexes {
		if i >= maxIndexDisplay {
			break
		}

		detailKey := fmt.Sprintf("index_%d", i+1)
		health.Details[detailKey] = map[string]any{
			"name":      index.Name,
			"dimension": index.Dimension,
			"metric":    index.Metric,
			"host":      index.Host,
			"spec":      index.Spec,
			"status":    index.Status,
		}
	}
}
