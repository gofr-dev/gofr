package eventhub

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/checkpoints"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// code reference from https://learn.microsoft.com/en-us/azure/event-hubs/event-hubs-go-get-started-send
// metrics are being registered in the container, and we are using the same metrics so we have not re-registered the metrics here.
// It is different from other datasources.

var (
	ErrNoMsgReceived      = errors.New("no message received")
	ErrTopicMismatch      = errors.New("topic should be same as Event Hub name")
	errClientNotConnected = errors.New("eventhub client not connected")
	errEmptyTopic         = errors.New("topic name cannot be empty")
)

const (
	defaultQueryTimeout     = 30 * time.Second
	eventHubPropsTimeout    = 2 * time.Second
	basicTierMaxPartitions  = 2
	basicTierReceiveTimeout = 3 * time.Second
)

type Config struct {
	ConnectionString          string
	ContainerConnectionString string
	StorageServiceURL         string
	StorageContainerName      string
	EventhubName              string
	// if not provided, it will read from the $Default consumergroup.
	ConsumerGroup string
	// the following configs are for advance setup of the Event Hub.
	StorageOptions   *container.ClientOptions
	BlobStoreOptions *checkpoints.BlobStoreOptions
	ConsumerOptions  *azeventhubs.ConsumerClientOptions
	ProducerOptions  *azeventhubs.ProducerClientOptions
}

type Client struct {
	producer *azeventhubs.ProducerClient
	consumer *azeventhubs.ConsumerClient
	// we are using a processor such that to keep consuming the events from all the different partitions.
	processor *azeventhubs.Processor
	// a checkpoint is being called while committing the event received from the event.
	checkPoint *checkpoints.BlobStore
	// processorCtx is being stored such that to gracefully shutting down the application.
	processorCtx context.CancelFunc
	cfg          Config
	logger       Logger
	metrics      Metrics
	tracer       trace.Tracer
}

// New Creates the client for Event Hub.
//
//nolint:gocritic // cfg is a configuration struct.
func New(cfg Config) *Client {
	return &Client{
		cfg: cfg,
	}
}

//nolint:gocritic // cfg is a configuration struct.
func (c *Client) validConfigs(cfg Config) bool {
	ok := true

	if cfg.EventhubName == "" {
		ok = false

		c.logger.Error("eventhubName cannot be an empty")
	}

	if cfg.ConnectionString == "" {
		ok = false

		c.logger.Error("connectionString cannot be an empty")
	}

	if cfg.StorageServiceURL == "" {
		ok = false

		c.logger.Error("storageServiceURL cannot be an empty")
	}

	if cfg.StorageContainerName == "" {
		ok = false

		c.logger.Error("storageContainerName cannot be an empty")
	}

	if cfg.ContainerConnectionString == "" {
		ok = false

		c.logger.Error("containerConnectionString cannot be an empty")
	}

	return ok
}

// UseLogger sets the logger for the Event Hub client.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the Event Hub client.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for the Event Hub client.
func (c *Client) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

// Connect establishes a connection to Event Hub and registers metrics using the provided configuration when the client was Created.
func (c *Client) Connect() {
	if !c.validConfigs(c.cfg) {
		return
	}

	if c.cfg.ConsumerGroup == "" {
		c.cfg.ConsumerGroup = azeventhubs.DefaultConsumerGroup
		c.logger.Debugf("Using default consumer group: %s", c.cfg.ConsumerGroup)
	} else {
		c.logger.Debugf("Using provided consumer group: %s", c.cfg.ConsumerGroup)
	}

	c.logger.Debug("Event Hub connection started using connection string")

	producerClient, err := azeventhubs.NewProducerClientFromConnectionString(c.cfg.ConnectionString,
		c.cfg.EventhubName, c.cfg.ProducerOptions)
	if err != nil {
		c.logger.Errorf("error occurred while creating producer client %v", err)

		return
	}

	c.logger.Debug("Event Hub producer client setup success")

	containerClient, err := container.NewClientFromConnectionString(c.cfg.ContainerConnectionString, c.cfg.StorageContainerName,
		c.cfg.StorageOptions)
	if err != nil {
		c.logger.Errorf("error occurred while creating container client %v", err)

		return
	}

	c.logger.Debug("Event Hub container client setup success")

	// create a checkpoint store that will be used by the event hub
	checkpointStore, err := checkpoints.NewBlobStore(containerClient, c.cfg.BlobStoreOptions)
	if err != nil {
		c.logger.Errorf("error occurred while creating blobstore %v", err)

		return
	}

	c.logger.Debug("Event Hub blobstore client setup success")

	// create a consumer client using a connection string to the namespace and the event hub
	consumerClient, err := azeventhubs.NewConsumerClientFromConnectionString(c.cfg.ConnectionString, c.cfg.EventhubName,
		c.cfg.ConsumerGroup, c.cfg.ConsumerOptions)
	if err != nil {
		c.logger.Errorf("error occurred while creating consumer client %v", err)

		return
	}

	c.logger.Debug("Event Hub consumer client setup success")

	// create a processor to receive and process events
	processor, err := azeventhubs.NewProcessor(consumerClient, checkpointStore, nil)
	if err != nil {
		c.logger.Errorf("error occurred while creating processor %v", err)

		return
	}

	c.logger.Debug("Event Hub processor setup success")

	processorCtx, processorCancel := context.WithCancel(context.TODO())
	c.processorCtx = processorCancel

	// it is being run in a go-routine as it is a never ending process and has to be kept running to subscribe to events.
	go func() {
		if err = processor.Run(processorCtx); err != nil {
			c.logger.Errorf("error occurred while running processor %v", err)

			return
		}

		c.logger.Debug("Event Hub processor running successfully")
	}()

	c.processor = processor
	c.producer = producerClient
	c.consumer = consumerClient
	c.checkPoint = checkpointStore

	c.logger.Debug("Event Hub client initialization complete")
}

// Subscribe checks all partitions for the first available event and returns it.
func (c *Client) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	if c.producer == nil || c.consumer == nil || c.processor == nil {
		return nil, errClientNotConnected
	}

	// Try processor approach first
	partitionClient := c.processor.NextPartitionClient(ctx)
	if partitionClient != nil {
		return c.processEventsFromPartitionClient(ctx, topic, partitionClient)
	}

	// Fallback to direct consumer approach if processor doesn't have partition clients ready
	return c.subscribeDirectFromConsumer(ctx, topic)
}

// processEventsFromPartitionClient processes events using the processor partition client.
func (c *Client) processEventsFromPartitionClient(ctx context.Context, topic string,
	partitionClient *azeventhubs.ProcessorPartitionClient) (*pubsub.Message, error) {
	defer closePartitionResources(ctx, partitionClient)

	timeout := c.getReceiveTimeout()

	receiveCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "topic", topic, "subscription_name", partitionClient.PartitionID())

	start := time.Now()

	// ReceiveEvents signature: ReceiveEvents(ctx context.Context, count int, options *ReceiveEventsOptions) ([]*ReceivedEventData, error)
	// Note: ReceiveEventsOptions is nil for default behavior.
	events, err := partitionClient.ReceiveEvents(receiveCtx, 1, nil)
	if err != nil {
		if !errors.Is(err, context.DeadlineExceeded) {
			c.logger.Debugf("Error receiving events from partition %s: %v", partitionClient.PartitionID(), err)
		}

		return nil, nil
	}

	if len(events) == 0 {
		return nil, nil
	}

	// Create message from the first event
	msg := pubsub.NewMessage(ctx)
	msg.Value = events[0].Body
	msg.Committer = &Message{
		event:     events[0],
		processor: partitionClient,
		logger:    c.logger,
	}
	msg.Topic = topic
	msg.MetaData = events[0].EventData

	end := time.Since(start)
	c.logger.Debug(&Log{
		Mode:          "SUB",
		MessageValue:  strings.Join(strings.Fields(string(msg.Value)), " "),
		Topic:         topic,
		Host:          c.cfg.EventhubName + ":" + c.cfg.ConsumerGroup + ":" + partitionClient.PartitionID(),
		PubSubBackend: "EVHUB",
		Time:          end.Microseconds(),
	})

	c.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_success_count", "topic", topic, "subscription_name", partitionClient.PartitionID())

	return msg, nil
}

// subscribeDirectFromConsumer uses consumer client directly as fallback.
func (c *Client) subscribeDirectFromConsumer(ctx context.Context, topic string) (*pubsub.Message, error) {
	// Get partition information
	props, err := c.consumer.GetEventHubProperties(ctx, nil)
	if err != nil {
		c.logger.Errorf("Failed to get Event Hub properties: %v", err)
		return nil, err
	}

	// Try each partition for available messages - use LATEST to avoid old messages
	for _, partitionID := range props.PartitionIDs {
		msg, err := c.tryReadFromPartition(ctx, partitionID, topic)
		if err != nil {
			c.logger.Debugf("Error reading from partition %s: %v", partitionID, err)
			continue
		}

		if msg != nil {
			return msg, nil
		}
	}

	return nil, nil
}

// tryReadFromPartition attempts to read a single message from specified partition.
func (c *Client) tryReadFromPartition(ctx context.Context, partitionID, topic string) (*pubsub.Message, error) {
	// Create partition client for direct read with LATEST position to avoid old messages.
	partitionClient, err := c.consumer.NewPartitionClient(partitionID, &azeventhubs.PartitionClientOptions{
		StartPosition: azeventhubs.StartPosition{
			Latest: to.Ptr(true), // Use Latest to only get new messages
		},
	})
	if err != nil {
		return nil, err
	}

	defer partitionClient.Close(ctx)

	timeout := c.getReceiveTimeout()

	receiveCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "topic", topic, "subscription_name", partitionID)

	start := time.Now()

	events, err := partitionClient.ReceiveEvents(receiveCtx, 1, nil)

	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}

	if len(events) == 0 {
		return nil, nil // No message available in this partition
	}

	// Create message from event
	msg := pubsub.NewMessage(ctx)

	msg.Value = events[0].Body
	msg.Committer = &Message{
		event:     events[0],
		processor: nil, // Not using processor for direct reads
		logger:    c.logger,
	}
	msg.Topic = topic
	msg.MetaData = events[0].EventData

	end := time.Since(start)
	c.logger.Debug(&Log{
		Mode:          "SUB",
		MessageValue:  strings.Join(strings.Fields(string(msg.Value)), " "),
		Topic:         topic,
		Host:          c.cfg.EventhubName + ":" + c.cfg.ConsumerGroup + ":" + partitionID,
		PubSubBackend: "EVHUB",
		Time:          end.Microseconds(),
	})

	c.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_success_count", "topic", topic, "subscription_name", partitionID)

	return msg, nil
}

// getReceiveTimeout returns appropriate timeout based on Event Hub characteristics.
func (c *Client) getReceiveTimeout() time.Duration {
	// Check if this might be basic tier by examining partition count
	if c.isLikelyBasicTier() {
		return basicTierReceiveTimeout
	}

	return time.Second
}

// isLikelyBasicTier detects basic tier characteristics.
func (c *Client) isLikelyBasicTier() bool {
	if c.consumer == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), eventHubPropsTimeout)
	defer cancel()

	props, err := c.consumer.GetEventHubProperties(ctx, nil)
	if err != nil {
		return false // Default to standard behavior on error
	}

	// Basic tier typically has fewer partitions
	return len(props.PartitionIDs) <= basicTierMaxPartitions
}

func closePartitionResources(ctx context.Context, partitionClient *azeventhubs.ProcessorPartitionClient) {
	partitionClient.Close(ctx)
}

func (c *Client) Publish(ctx context.Context, topic string, message []byte) error {
	if topic != c.cfg.EventhubName {
		return ErrTopicMismatch
	}

	c.metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", topic)

	newBatchOptions := &azeventhubs.EventDataBatchOptions{}

	batch, err := c.producer.NewEventDataBatch(ctx, newBatchOptions)
	if err != nil {
		c.logger.Errorf("failed to create event batch %v", err)

		return err
	}

	data := []*azeventhubs.EventData{{
		Body: message,
	}}

	for i := 0; i < len(data); i++ {
		err = batch.AddEventData(data[i], nil)
		if err != nil {
			c.logger.Debugf("failed to add event data to batch %v", err)
		}
	}

	start := time.Now()

	// send the batch of events to the event hub
	if err := c.producer.SendEventDataBatch(ctx, batch, nil); err != nil {
		return err
	}

	end := time.Since(start)

	c.logger.Debug(&Log{
		Mode:          "PUB",
		MessageValue:  strings.Join(strings.Fields(string(message)), " "),
		Topic:         topic,
		Host:          c.cfg.EventhubName,
		PubSubBackend: "EVHUB",
		Time:          end.Microseconds(),
	})

	c.metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", topic)

	return nil
}

func (c *Client) Health() datasource.Health {
	c.logger.Error("health-check not implemented for Event Hub")

	return datasource.Health{}
}

func (c *Client) CreateTopic(_ context.Context, name string) error {
	// For Event Hub, creating topics is not supported, but we don't want to fail migrations
	if name == "gofr_migrations" {
		return nil
	}

	c.logger.Error("topic creation is not supported in Event Hub")

	return nil
}

func (c *Client) DeleteTopic(context.Context, string) error {
	c.logger.Error("topic deletion is not supported in Event Hub")

	return nil
}

// Query retrieves messages from Azure Event Hub.
func (c *Client) Query(ctx context.Context, query string, args ...any) ([]byte, error) {
	if c.consumer == nil {
		return nil, errClientNotConnected
	}

	if query == "" {
		return nil, errEmptyTopic
	}

	if query != c.cfg.EventhubName {
		return nil, ErrTopicMismatch
	}

	startPosition, limit := c.parseQueryArgs(args...)

	// Use provided context or add default timeout
	readCtx := ctx

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc

		readCtx, cancel = context.WithTimeout(ctx, defaultQueryTimeout)
		defer cancel()
	}

	return c.readMessages(readCtx, startPosition, limit)
}

func (c *Client) GetEventHubName() string {
	return c.cfg.EventhubName
}

// Close safely closes all Event Hub clients and resources.
func (c *Client) Close() error {
	var lastErr error

	// Close producer if it exists
	if c.producer != nil {
		if err := c.producer.Close(context.Background()); err != nil {
			c.logger.Errorf("failed to close Event Hub producer: %v", err)
			lastErr = err
		}
	}

	// Close consumer if it exists
	if c.consumer != nil {
		if err := c.consumer.Close(context.Background()); err != nil {
			c.logger.Errorf("failed to close Event Hub consumer: %v", err)
			lastErr = err
		}
	}

	// Cancel processor context if it exists
	if c.processorCtx != nil {
		c.processorCtx()
		c.logger.Debug("Event Hub processor context canceled")
	}

	return lastErr
}
