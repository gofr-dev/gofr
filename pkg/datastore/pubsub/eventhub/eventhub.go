// Package eventhub provides methods to interact, publish and consume events from Azure Eventhub
package eventhub

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"

	"github.com/Azure/azure-amqp-common-go/v3/aad"
	"github.com/Azure/azure-amqp-common-go/v3/sas"
	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"github.com/Azure/go-autorest/autorest/azure"

	"gofr.dev/pkg"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/datastore/pubsub/avro"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

// Config contains configuration options for interacting with Azure Eventhub.
type Config struct {
	Namespace        string
	EventhubName     string
	ClientID         string
	ClientSecret     string
	TenantID         string
	SharedAccessName string
	SharedAccessKey  string
	// Offsets is slice of Offset in which "PartitionID" and "StartOffset"
	// are the field needed to be set to start consuming from specific offset
	Offsets           []Offset
	ConnRetryDuration int
}

// AvroWithEventhubConfig combines the Azure Eventhub configuration and Avro configuration.
type AvroWithEventhubConfig struct {
	EventhubConfig Config
	AvroConfig     avro.Config
}

// Eventhub represents a client to interact with Azure Eventhub.
type Eventhub struct {
	Config
	hub                *eventhub.Hub
	partitionOffsetMap map[string]string // for persisting offsets
	initialiseOffset   sync.Once
}

// Offset specifies the partition and starting offset for consuming events.
type Offset struct {
	PartitionID string
	StartOffset string
}

// New returns new client
func New(c *Config) (pubsub.PublisherSubscriber, error) {
	pubsub.RegisterMetrics()

	if c.SharedAccessKey != "" && c.SharedAccessName != "" {
		tokenProviderOption := sas.TokenProviderWithKey(c.SharedAccessName, c.SharedAccessKey)

		tokenProvider, err := sas.NewTokenProvider(tokenProviderOption)
		if err != nil {
			return &Eventhub{Config: *c}, err
		}

		hub, err := eventhub.NewHub(c.Namespace, c.EventhubName, tokenProvider)
		if err != nil {
			return &Eventhub{Config: *c}, err
		}

		return &Eventhub{hub: hub, Config: *c, partitionOffsetMap: make(map[string]string)}, nil
	}

	jwtProvider, err := aad.NewJWTProvider(jwtProvider(c))
	if err != nil {
		return &Eventhub{Config: *c}, err
	}

	hub, err := eventhub.NewHub(c.Namespace, c.EventhubName, jwtProvider)
	if err != nil {
		return &Eventhub{Config: *c}, err
	}

	return &Eventhub{hub: hub, Config: *c, partitionOffsetMap: make(map[string]string)}, nil
}

func jwtProvider(c *Config) aad.JWTProviderOption {
	return func(config *aad.TokenProviderConfiguration) error {
		config.TenantID = c.TenantID
		config.ClientID = c.ClientID
		config.ClientSecret = c.ClientSecret
		config.Env = &azure.PublicCloud

		return nil
	}
}

// PublishEvent publishes the event to eventhub
func (e *Eventhub) PublishEvent(key string, value interface{}, headers map[string]string) (err error) {
	return e.PublishEventWithOptions(key, value, headers, nil)
}

// PublishEventWithOptions publishes message to eventhub. Ability to provide additional options described in PublishOptions struct
func (e *Eventhub) PublishEventWithOptions(key string, value interface{}, _ map[string]string,
	_ *pubsub.PublishOptions) (err error) {
	// for every publish
		pubsub.PublishTotalCount(e.EventhubName, "")

	data, ok := value.([]byte)
	if !ok {
		data, err = json.Marshal(value)
		if err != nil {
			// for unsuccessful publish 
			pubsub.PublishFailureCount(e.EventhubName, "")

			return err
		}
	}

	event := eventhub.NewEvent(data)

	err = e.hub.Send(context.TODO(), event, eventhub.SendWithMessageID(key))
	if err != nil {
		// for unsuccessful publish 
		pubsub.PublishFailureCount(e.EventhubName, "")

		return err
	}

	// for successful publish 
	pubsub.PublishSuccessCount(e.EventhubName, "")

	return nil
}

// Subscribe read messages from eventhub configured
func (e *Eventhub) Subscribe() (*pubsub.Message, error) {
	// for every subscribe
	pubsub.SubscribeReceiveCount(e.EventhubName, "")

	msg := make(chan *pubsub.Message)

	handler := func(ctx context.Context, event *eventhub.Event) error {
		var partition int

		if event.SystemProperties.PartitionID != nil {
			partition = int(*event.SystemProperties.PartitionID)
		}

		msg <- &pubsub.Message{
			Value:     string(event.Data),
			Partition: partition,
			Offset:    *event.SystemProperties.Offset,
			Topic:     e.EventhubName,
		}

		e.partitionOffsetMap[strconv.Itoa(partition)] = strconv.Itoa(int(*event.SystemProperties.Offset))

		return nil
	}

	ctx := context.TODO()

	runtimeInfo, err := e.hub.GetRuntimeInformation(ctx)
	if err != nil {
		// for failed subscribe
		pubsub.SubscribeFailureCount(e.EventhubName, "")
		return nil, err
	}

	// Set the initial offset value for subscribe
	if e.Offsets != nil {
		e.initialiseOffset.Do(func() {
			for _, offset := range e.Offsets {
				e.partitionOffsetMap[offset.PartitionID] = offset.StartOffset
			}
		})
	}

	for _, partitionID := range runtimeInfo.PartitionIDs {
		offset := e.partitionOffsetMap[partitionID]

		_, err := e.hub.Receive(ctx, partitionID, handler, eventhub.ReceiveWithStartingOffset(offset))
		if err != nil {
			// for failed subscribe
			pubsub.SubscribeFailureCount(e.EventhubName, "")
			return nil, err
		}
	}
	// for successful subscribe
	pubsub.SubscribeSuccessCount(e.EventhubName, "")

	return <-msg, nil
}

/*
SubscribeWithCommit calls the CommitFunc after subscribing message from eventhub and based on the return values decides
whether to commit message and consume another message
*/
func (e *Eventhub) SubscribeWithCommit(pubsub.CommitFunc) (*pubsub.Message, error) {
	return e.Subscribe()
}

// Bind parses the encoded data and stores the result in the value pointed to by target
func (e *Eventhub) Bind(message []byte, target interface{}) error {
	return json.Unmarshal(message, target)
}

// Ping checks for the health of eventhub, returns an error if it is down
func (e *Eventhub) Ping() error {
	_, err := e.hub.GetRuntimeInformation(context.TODO())
	return err
}

// HealthCheck returns the health of the PubSub
func (e *Eventhub) HealthCheck() types.Health {
	// handling nil object
	if e == nil {
		return types.Health{
			Name:   datastore.EventHub,
			Status: pkg.StatusDown,
		}
	}

	resp := types.Health{
		Name:     datastore.EventHub,
		Status:   pkg.StatusDown,
		Host:     e.Namespace,
		Database: e.EventhubName,
	}

	// configs is present but not connected
	if e.hub == nil {
		return resp
	}

	if err := e.Ping(); err != nil {
		return resp
	}

	resp.Status = pkg.StatusUp

	return resp
}

// CommitOffset not implemented for Eventhub
func (e *Eventhub) CommitOffset(pubsub.TopicPartition) {
}

// IsSet checks whether eventhub is initialized or not
func (e *Eventhub) IsSet() bool {
	if e == nil {
		return false
	}

	return e.hub != nil
}

// NewEventHubWithAvro initialize EventHub with Avro when EventHubConfig and AvroConfig are right
func NewEventHubWithAvro(config *AvroWithEventhubConfig, logger log.Logger) (pubsub.PublisherSubscriber, error) {
	eventHub, err := New(&config.EventhubConfig)
	if err != nil {
		logger.Errorf("Eventhub cannot be initialized, err: %v", err)
		return nil, err
	}

	p, err := avro.NewWithConfig(&config.AvroConfig, eventHub)
	if err != nil {
		logger.Errorf("Avro cannot be initialized, err: %v", err)
		return nil, err
	}

	return p, nil
}
