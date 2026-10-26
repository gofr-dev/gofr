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

func TestKafkaClient_PublishWithKey(t *testing.T) {
	for _, key := range [][]byte{nil, {}, []byte("organization:ticket:command")} {
		t.Run(string(key), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			writer := NewMockWriter(ctrl)
			metrics := NewMockMetrics(ctrl)
			client := &kafkaClient{
				writer: writer, metrics: metrics, logger: logging.NewMockLogger(logging.DEBUG),
				config: Config{Brokers: []string{"localhost:9092"}},
			}

			var publisher pubsub.KeyedPublisher = client

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			payload := []byte("payload")

			writer.EXPECT().WriteMessages(gomock.Any(), gomock.Any()).DoAndReturn(
				func(publishCtx context.Context, messages ...kafkago.Message) error {
					assert.Equal(t, ctx.Done(), publishCtx.Done())
					require.Len(t, messages, 1)
					assert.Equal(t, "topic", messages[0].Topic)
					assert.Equal(t, key, messages[0].Key)
					assert.Equal(t, payload, messages[0].Value)
					assert.False(t, messages[0].Time.IsZero())

					return nil
				},
			)
			metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", "topic")
			metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_success_count", "topic", "topic")
			require.NoError(t, publisher.PublishWithKey(ctx, "topic", key, payload))
		})
	}
}

func TestKafkaClient_PublishWithKeyError(t *testing.T) {
	ctrl := gomock.NewController(t)
	writer := NewMockWriter(ctrl)
	metrics := NewMockMetrics(ctrl)
	publishErr := testutil.CustomError{ErrorMessage: "publish failed"}
	client := &kafkaClient{writer: writer, metrics: metrics, logger: logging.NewMockLogger(logging.DEBUG)}
	writer.EXPECT().WriteMessages(gomock.Any(), gomock.Any()).Return(publishErr)
	metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", "topic")
	require.ErrorIs(t, client.PublishWithKey(t.Context(), "topic", []byte("key"), []byte("payload")), publishErr)
}

func TestKafkaWriterKeyPartitioning(t *testing.T) {
	writer := createKafkaWriter(&Config{
		Brokers: []string{"localhost:9092"}, BatchSize: 1, BatchBytes: DefaultBatchBytes, BatchTimeout: DefaultBatchTimeout,
	}, &kafkago.Dialer{}, logging.NewMockLogger(logging.DEBUG))

	t.Cleanup(func() { require.NoError(t, writer.Close()) })

	concrete, ok := writer.(*kafkago.Writer)
	require.True(t, ok)
	balancer, ok := concrete.Balancer.(*kafkago.Hash)
	require.True(t, ok)

	keyed := kafkago.Message{Key: []byte("organization:ticket:command")}
	partition := balancer.Balance(keyed, 0, 1, 2)

	for range 10 {
		assert.Equal(t, partition, balancer.Balance(keyed, 0, 1, 2))
	}

	seen := make(map[int]bool)
	for range 3 {
		seen[balancer.Balance(kafkago.Message{}, 0, 1, 2)] = true
	}

	assert.Len(t, seen, 3, "nil keys must preserve round-robin routing")
}
