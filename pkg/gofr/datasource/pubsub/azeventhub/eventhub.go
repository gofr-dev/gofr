package azeventhub

import (
	"context"
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/checkpoints"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

type Config struct {
	ConnectionString          string
	ContainerConnectionString string
	StorageServiceURL         string
	StorageContainerName      string
	EventhubName              string
	ConsumerGroup             string
	StorageOptions            *container.ClientOptions
	BlobStoreOptions          *checkpoints.BlobStoreOptions
	ConsumerOptions           *azeventhubs.ConsumerClientOptions
	ProducerOptions           *azeventhubs.ProducerClientOptions
}

type Client struct {
	producer     *azeventhubs.ProducerClient
	consumer     *azeventhubs.ConsumerClient
	processor    *azeventhubs.Processor
	checkPoint   *checkpoints.BlobStore
	processorCtx context.CancelFunc
	cfg          Config
	logger       Logger
	metrics      Metrics
}

type azMessage struct {
	event     *azeventhubs.ReceivedEventData
	processor *azeventhubs.ProcessorPartitionClient
}

func (a *azMessage) Commit() {
	// Update the checkpoint with the latest event received
	a.processor.UpdateCheckpoint(context.Background(), a.event, nil)
}

func New(cfg Config) *Client {
	return &Client{
		cfg: cfg,
	}
}

func (c *Client) validConfigs(cfg Config) bool {
	ok := true

	if cfg.EventhubName == "" {
		ok = false

		c.logger.Error("EventhubName cannot be an empty")
	}

	if cfg.ConnectionString == "" {
		ok = false

		c.logger.Error("ConnectionString cannot be an empty")
	}

	if cfg.StorageServiceURL == "" {
		ok = false

		c.logger.Error("StorageServiceURL cannot be an empty")
	}

	if cfg.StorageContainerName == "" {
		ok = false

		c.logger.Error("StorageContainerName cannot be an empty")
	}

	if cfg.ContainerConnectionString == "" {
		ok = false

		c.logger.Error("ContainerConnectionString cannot be an empty")
	}

	if cfg.ConsumerGroup == "" {
		cfg.ConsumerGroup = azeventhubs.DefaultConsumerGroup
	}

	return ok

}

// UseLogger sets the logger for the Cassandra client.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the Cassandra client.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// Connect establishes a connection to Cassandra and registers metrics using the provided configuration when the client was Created.
func (c *Client) Connect() {
	if !c.validConfigs(c.cfg) {
		return
	}

	producerClient, err := azeventhubs.NewProducerClientFromConnectionString(c.cfg.ConnectionString,
		c.cfg.EventhubName, c.cfg.ProducerOptions)
	if err != nil {
		fmt.Println(err)
		c.logger.Error(fmt.Sprintf("error occurred while creating producer client %v", err))

		return
	}

	containerClient, err := container.NewClientFromConnectionString(c.cfg.ContainerConnectionString, c.cfg.StorageContainerName,
		c.cfg.StorageOptions)
	if err != nil {
		c.logger.Error(fmt.Sprintf("error occurred while creating container client %v", err))

		return
	}

	// create a checkpoint store that will be used by the event hub
	checkpointStore, err := checkpoints.NewBlobStore(containerClient, c.cfg.BlobStoreOptions)
	if err != nil {
		c.logger.Errorf("error occurred while creating blobstore %v", err)

		return
	}

	// create a consumer client using a connection string to the namespace and the event hub
	consumerClient, err := azeventhubs.NewConsumerClientFromConnectionString(c.cfg.ConnectionString, c.cfg.EventhubName,
		c.cfg.ConsumerGroup, c.cfg.ConsumerOptions)
	if err != nil {
		c.logger.Errorf("error occurred while creating consumer client %v", err)

		return
	}

	// create a processor to receive and process events
	processor, err := azeventhubs.NewProcessor(consumerClient, checkpointStore, nil)
	if err != nil {
		c.logger.Errorf("error occurred while creating processor %v", err)

		return
	}

	processorCtx, processorCancel := context.WithCancel(context.TODO())
	c.processorCtx = processorCancel

	go func() {
		if err = processor.Run(processorCtx); err != nil {
			c.logger.Errorf("error occurred while running processor %v", err)

			return
		}
	}()

	c.processor = processor
	c.producer = producerClient
	c.consumer = consumerClient
	c.checkPoint = checkpointStore
}

// Subscribe checks all partitions for the first available event and returns it.
func (c *Client) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	if topic != c.cfg.EventhubName {
		return nil, errors.New("topic should be same as eventhub name")
	}

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

		msg, err = c.processEvents(ctx, partitionClient)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		return msg, nil
	}

	return nil, nil
}

func (c *Client) processEvents(ctx context.Context, partitionClient *azeventhubs.ProcessorPartitionClient) (*pubsub.Message, error) {
	defer closePartitionResources(ctx, partitionClient)
	for {
		events, err := partitionClient.ReceiveEvents(ctx, 1, nil)

		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		msg := pubsub.NewMessage(ctx)

		if len(events) == 1 {
			msg.Value = events[0].Body
			msg.Committer = &azMessage{
				event:     events[0],
				processor: partitionClient,
			}
			msg.Topic = partitionClient.PartitionID()
			msg.MetaData = events[0]

			return msg, nil
		}

		if len(events) != 0 {
			if err = partitionClient.UpdateCheckpoint(ctx, events[len(events)-1], nil); err != nil {
				return nil, err
			}
		}
	}
}

func closePartitionResources(ctx context.Context, partitionClient *azeventhubs.ProcessorPartitionClient) {
	defer partitionClient.Close(ctx)
}

func (c *Client) Publish(ctx context.Context, topic string, message []byte) error {
	if topic != c.cfg.EventhubName {
		return errors.New("topic should be same as eventhub name")
	}

	newBatchOptions := &azeventhubs.EventDataBatchOptions{}

	batch, err := c.producer.NewEventDataBatch(ctx, newBatchOptions)
	if err != nil {
		panic(err)
	}

	data := []*azeventhubs.EventData{{
		Body: message,
	}}

	for i := 0; i < len(data); i++ {
		err = batch.AddEventData(data[i], nil)
	}

	// send the batch of events to the event hub
	return c.producer.SendEventDataBatch(ctx, batch, nil)
}

func (c *Client) Health() datasource.Health {
	c.logger.Errorf("health-check not implemented for eventhub")

	return datasource.Health{}
}

func (c *Client) CreateTopic(context.Context, string) error {
	c.logger.Errorf("topic creation is not supported in eventhub")

	return nil
}

func (c *Client) DeleteTopic(context context.Context, name string) error {
	c.logger.Errorf("topic deletion is not supported in eventhub")

	return nil
}

func (c *Client) Close() error {
	err := c.producer.Close(context.Background())
	if err != nil {
		c.logger.Errorf("failed to close eventhub producer %v", err)
	}

	err = c.consumer.Close(context.Background())
	if err != nil {
		c.logger.Errorf("failed to close eventhub consumer %v", err)
	}

	c.processorCtx()

	return err
}
