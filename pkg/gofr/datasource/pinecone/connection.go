package pinecone

import (
	"github.com/pinecone-io/go-pinecone/v3/pinecone"
)

// connectionManager handles connection lifecycle
type connectionManager struct {
	client *Client
}

// newConnectionManager creates a new connection manager
func newConnectionManager(client *Client) *connectionManager {
	return &connectionManager{client: client}
}

// connect establishes a connection to Pinecone
func (cm *connectionManager) connect() {
	cm.logConnection()

	if !cm.validateAPIKey() {
		return
	}

	cm.setupMetrics()

	if err := cm.createPineconeClient(); err != nil {
		cm.logConnectionError(err)
		return
	}

	cm.client.connected = true
	cm.logSuccessfulConnection()
}

// logConnection logs the connection attempt
func (cm *connectionManager) logConnection() {
	if cm.client.logger != nil {
		cm.client.logger.Debugf("connecting to Pinecone with API key")
	}
}

// validateAPIKey checks if API key is provided
func (cm *connectionManager) validateAPIKey() bool {
	if cm.client.config.APIKey == "" {
		if cm.client.logger != nil {
			cm.client.logger.Errorf("API key is required for Pinecone connection")
		}
		return false
	}
	return true
}

// setupMetrics initializes metrics if available
func (cm *connectionManager) setupMetrics() {
	if cm.client.metrics == nil {
		return
	}

	config := getDefaultMetricsConfig()
	cm.client.metrics.NewHistogram(metricsHistogramName, metricsDescription, config.histogramBuckets...)
	cm.client.metrics.NewGauge(metricsGaugeName, gaugeDescription)
}

// createPineconeClient creates the Pinecone SDK client
func (cm *connectionManager) createPineconeClient() error {
	client, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: cm.client.config.APIKey,
	})
	if err != nil {
		return err
	}

	cm.client.client = client
	return nil
}

// logConnectionError logs connection errors
func (cm *connectionManager) logConnectionError(err error) {
	if cm.client.logger != nil {
		cm.client.logger.Errorf("failed to create Pinecone client: %v", err)
	}
}

// logSuccessfulConnection logs successful connection
func (cm *connectionManager) logSuccessfulConnection() {
	if cm.client.logger != nil {
		cm.client.logger.Infof("connected to Pinecone successfully")
	}
}
