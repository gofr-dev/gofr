package kafka

import (
	"context"
	"github.com/segmentio/kafka-go"

	"gofr.dev/pkg/gofr/datasource/pubsub"
)

type Config struct {
	Broker    string
	Partition int
}

type kafkaClient struct {
	conn   *kafka.Conn
	logger pubsub.Logger
}

func New(conf Config) *kafkaClient {
	conn, err := kafka.Dial("tcp", conf.Broker)
	if err != nil {
		return &kafkaClient{}
	}

	return &kafkaClient{
		conn: conn,
	}
}

func (k *kafkaClient) Publish(ctx context.Context, topic string, message interface{}) error {
	return nil
}

func (k *kafkaClient) Subscribe(ctx context.Context, topic string) pubsub.Message {
	var msg pubsub.Message

	return msg
}
