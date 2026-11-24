package kafka

import (
	"context"
	"testing"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestPartitionKeyWriterRetainsRoundRobinForUnkeyedMessages(t *testing.T) {
	writer := createKafkaWriter(&Config{
		Brokers: []string{"localhost:9092"}, BatchSize: 1,
		BatchBytes: DefaultBatchBytes, BatchTimeout: DefaultBatchTimeout,
	}, &kafkago.Dialer{}, logging.NewMockLogger(logging.DEBUG))

	t.Cleanup(func() { require.NoError(t, writer.Close()) })

	concrete, ok := writer.(*kafkago.Writer)
	require.True(t, ok)
	balancer, ok := concrete.Balancer.(*kafkago.Hash)
	require.True(t, ok)

	roundRobin := &kafkago.RoundRobin{}
	for range 6 {
		assert.Equal(t, roundRobin.Balance(kafkago.Message{}, 0, 1, 2),
			balancer.Balance(kafkago.Message{}, 0, 1, 2))
	}
}

func TestPublishExplicitlyUnkeyedOpaquePayload(t *testing.T) {
	for _, key := range [][]byte{nil, {}} {
		t.Run(string(key), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			writer, metrics := NewMockWriter(ctrl), NewMockMetrics(ctrl)
			client := &kafkaClient{
				writer: writer, metrics: metrics, logger: logging.NewMockLogger(logging.DEBUG),
				config: Config{Brokers: []string{"localhost:9092"}},
			}
			payload := []byte{0xff, 0x00, 0x81}

			writer.EXPECT().WriteMessages(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, messages ...kafkago.Message) error {
					require.Len(t, messages, 1)
					assert.Nil(t, messages[0].Key)
					assert.Equal(t, payload, messages[0].Value)

					return nil
				},
			)
			metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", "orders")
			metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_success_count", "topic", "orders")

			ctx := pubsub.WithPartitionKey(t.Context(), key)
			require.NoError(t, client.Publish(ctx, "orders", payload))
		})
	}
}

func TestPublishPartitionKeyWriteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	writer, metrics := NewMockWriter(ctrl), NewMockMetrics(ctrl)
	publishErr := testutil.CustomError{ErrorMessage: "publish failed"}
	client := &kafkaClient{writer: writer, metrics: metrics, logger: logging.NewMockLogger(logging.DEBUG)}
	writer.EXPECT().WriteMessages(gomock.Any(), gomock.Any()).Return(publishErr)
	metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", "orders")

	ctx := pubsub.WithPartitionKey(t.Context(), []byte("order-123"))
	require.ErrorIs(t, client.Publish(ctx, "orders", []byte("opaque payload")), publishErr)
}
