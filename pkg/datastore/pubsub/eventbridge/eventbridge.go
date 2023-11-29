// Package eventbridge provides methods to interact with AWS Eventbridge service allowing user to publish events to Eventbridge
package eventbridge

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eventbridge"

	"gofr.dev/pkg"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/gofr/types"
)

// Client represents a client to interact with AWS EventBridge.
type Client struct {
	client *eventbridge.EventBridge
	cfg    *Config
}

// Config stores the configuration parameters required to connect to AWS EventBridge.
type Config struct {
	ConnRetryDuration int
	EventBus          string
	EventSource       string
	Region            string
	AccessKeyID       string
	SecretAccessKey   string
}

type customProvider struct {
	keyID     string
	secretKey string
}

// Retrieve returns the credentials and error
func (cp customProvider) Retrieve() (credentials.Value, error) {
	return credentials.Value{AccessKeyID: cp.keyID, SecretAccessKey: cp.secretKey}, nil
}

// IsExpired returns false if expired
func (cp customProvider) IsExpired() bool {
	return false
}

// New returns new client
func New(cfg *Config) (*Client, error) {
	pubsub.RegisterMetrics()

	awsCfg := aws.NewConfig().WithRegion(cfg.Region)
	awsCfg.Credentials = credentials.NewCredentials(customProvider{cfg.AccessKeyID, cfg.SecretAccessKey})

	sess, err := session.NewSession(awsCfg)
	if err != nil {
		return nil, err
	}

	client := eventbridge.New(sess, awsCfg)

	return &Client{
		client: client,
		cfg:    cfg,
	}, nil
}

// PublishEvent publishes the event to eventbridge
func (c *Client) PublishEvent(detailType string, detail interface{}, _ map[string]string) error {
	// for every publish
	pubsub.PublishTotalCount(c.cfg.EventBus, "")

	payload, err := json.Marshal(detail)
	if err != nil {
		// for unsuccessful publish
		pubsub.PublishFailureCount(c.cfg.EventBus, "")
		return err
	}

	input := &eventbridge.PutEventsInput{
		Entries: []*eventbridge.PutEventsRequestEntry{
			{
				Detail:       aws.String(string(payload)),
				DetailType:   aws.String(detailType),
				EventBusName: aws.String(c.cfg.EventBus),
				Source:       aws.String(c.cfg.EventSource),
			},
		},
	}

	_, err = c.client.PutEvents(input)
	if err != nil {
		// for unsuccessful publish
		pubsub.PublishFailureCount(c.cfg.EventBus, "")
		return err
	}

	// for successful publish
	pubsub.PublishSuccessCount(c.cfg.EventBus, "")

	return nil
}

// PublishEventWithOptions not implemented for Eventbridge
func (c *Client) PublishEventWithOptions(string, interface{}, map[string]string,
	*pubsub.PublishOptions) (err error) {
	return nil
}

// Subscribe not implemented for Eventbridge
func (c *Client) Subscribe() (*pubsub.Message, error) {
	return nil, nil
}

// SubscribeWithCommit not implemented for Eventbridge
func (c *Client) SubscribeWithCommit(pubsub.CommitFunc) (*pubsub.Message, error) {
	return nil, nil
}

// Bind not implemented for Eventbridge
func (c *Client) Bind(message []byte, target interface{}) error {
	return json.Unmarshal(message, &target)
}

// Ping not implemented for Eventbridge
func (c *Client) Ping() error {
	return nil
}

// CommitOffset not implemented for Eventbridge
func (c *Client) CommitOffset(pubsub.TopicPartition) {

}

// HealthCheck checks eventbridge health.
func (c *Client) HealthCheck() types.Health {
	if c == nil {
		return types.Health{
			Name:   datastore.EventBridge,
			Status: pkg.StatusDown,
		}
	}

	resp := types.Health{
		Name:     datastore.EventBridge,
		Status:   pkg.StatusDown,
		Host:     c.cfg.Region,
		Database: c.cfg.EventBus,
	}

	if c.client == nil {
		return resp
	}

	resp.Status = pkg.StatusUp

	return resp
}

// IsSet checks whether eventbridge is initialized or not
func (c *Client) IsSet() bool {
	if c == nil {
		return false
	}

	if c.client == nil {
		return false
	}

	return true
}
