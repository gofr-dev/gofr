package datasource

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type Kafka interface {
	Publish(context.Context, kafka.Message) error
}
