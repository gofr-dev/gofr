package eventhub

import (
	"context"
	"errors"
	"strings"
	"time"

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
	ErrNoMsgReceived = errors.New("no message received")
	ErrTopicMismatch = errors.New("topic should be same as Event Hub name")
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

	if cfg.ConsumerGroup == "" {
		cfg.ConsumerGroup = azeventhubs.DefaultConsumerGroup
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

// Connect establishes a connection to Cassandra and registers metrics using the provided configuration when the client was Created.
func (c *Client) Connect() {
	if !c.validConfigs(c.cfg) {
		return
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
}

// Subscribe checks all partitions for the first available event and returns it.
func (c *Client) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	var (
		msg *pubsub.Message
		err error
	)

	//  for each partition in the event hub, create a partition client with processEvents as the function to process events
	for {
		partitionClient := c.processor.NextPartitionClient(ctx)

		if partitionClient == nil {
			break
		}

		c.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "topic", topic, "subscription_name", partitionClient.PartitionID())

		start := time.Now()

		select {
		case <-ctx.Done():
			return nil, nil
		default:
			msg, err = c.processEvents(ctx, partitionClient)
			if errors.Is(err, ErrNoMsgReceived) {
				// If no message is received, we don't achieve anything by returning error rather check in a different partition.
				// This logic may change if we remove the timeout while receiving a message. However, waiting on just one partition
				// might lead to missing data, so spawning one go-routine or having a worker pool can be an option to do this operation faster.
				continue
			}

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
	}

	return nil, nil
}

func (*Client) processEvents(ctx context.Context, partitionClient *azeventhubs.ProcessorPartitionClient) (*pubsub.Message, error) {
	defer closePartitionResources(ctx, partitionClient)

	receiveCtx, receiveCtxCancel := context.WithTimeout(ctx, time.Second)
	events, err := partitionClient.ReceiveEvents(receiveCtx, 1, nil)

	receiveCtxCancel()

	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}

	if len(events) == 0 {
		return nil, ErrNoMsgReceived
	}

	msg := pubsub.NewMessage(ctx)

	msg.Value = events[0].Body
	msg.Committer = &Message{
		event:     events[0],
		processor: partitionClient,
	}

	msg.Topic = partitionClient.PartitionID()
	msg.MetaData = events[0].EventData

	return msg, nil
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

func (c *Client) CreateTopic(context.Context, string) error {
	c.logger.Error("topic creation is not supported in Event Hub")

	return nil
}

func (c *Client) DeleteTopic(context.Context, string) error {
	c.logger.Error("topic deletion is not supported in Event Hub")

	return nil
}

func (c *Client) Close() error {
	err := c.producer.Close(context.Background())
	if err != nil {
		c.logger.Errorf("failed to close Event Hub producer %v", err)
	}

	err = c.consumer.Close(context.Background())
	if err != nil {
		c.logger.Errorf("failed to close Event Hub consumer %v", err)
	}

	c.processorCtx()

	return err
}
