package kafka

import (
	"context"
	"github.com/segmentio/kafka-go"

	"gofr.dev/pkg/gofr/datasource/pubsub"
)

type Config struct {
	Broker    string
	Partition int
	Offset    int64
}

type kafkaClient struct {
	conn   *kafka.Conn
	logger pubsub.Logger
}

func New(conf Config, logger pubsub.Logger) pubsub.Client {
	conn, err := kafka.Dial("tcp", conf.Broker)
	if err != nil {
		logger.Errorf("could not connect to Kafka at %v, error : %v", conf.Broker, err.Error())
		return &kafkaClient{
			logger: logger,
		}
	}

	if conf.Offset != 0 {
		conn.Seek(conf.Offset, kafka.SeekStart)
	}

	return &kafkaClient{
		conn:   conn,
		logger: logger,
	}
}

func (k *kafkaClient) Publish(ctx context.Context, topic string, message interface{}) error {
	return nil
}

func (k *kafkaClient) Subscribe(ctx context.Context, topic string) pubsub.Message {
	var msg pubsub.Message

	return msg
}
