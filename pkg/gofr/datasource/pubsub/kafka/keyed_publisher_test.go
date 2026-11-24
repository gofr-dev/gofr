package kafka

import (
	"context"
	"testing"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource/pubsub/internal/keyfields"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestKafkaClient_ConfiguredKey(t *testing.T) {
	for _, tt := range []struct {
		name, mapping, topic, payload string
		key                           []byte
	}{
		{"configured", "orders:orderId", "orders", `{"orderId":"order-123"}`, []byte("order-123")},
		{"other topic", "orders:orderId", "events", `{"orderId":"order-123"}`, nil},
		{"no config", "", "orders", `{"orderId":"order-123"}`, nil},
		{"missing field", "orders:orderId", "orders", `{}`, nil},
		{"not JSON", "orders:orderId", "orders", `payload`, nil},
		{"empty key", "orders:orderId", "orders", `{"orderId":""}`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			writer := NewMockWriter(ctrl)
			metrics := NewMockMetrics(ctrl)
			client := &kafkaClient{
				writer: writer, metrics: metrics, logger: logging.NewMockLogger(logging.DEBUG),
				config: Config{Brokers: []string{"localhost:9092"}}, keyFields: keyfields.Parse(tt.mapping),
			}
			balancer := &kafkago.Hash{}
			partitions := make([]int, 0, 6)

			writer.EXPECT().WriteMessages(gomock.Any(), gomock.Any()).Times(6).DoAndReturn(
				func(_ context.Context, messages ...kafkago.Message) error {
					require.Len(t, messages, 1)
					assert.Equal(t, tt.key, messages[0].Key)
					assert.Equal(t, []byte(tt.payload), messages[0].Value)
					assert.Equal(t, tt.topic, messages[0].Topic)
					partitions = append(partitions, balancer.Balance(messages[0], 0, 1, 2))

					return nil
				},
			)
			metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", tt.topic).Times(6)
			metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_success_count", "topic", tt.topic).Times(6)

			for range 6 {
				require.NoError(t, client.Publish(t.Context(), tt.topic, []byte(tt.payload)))
			}

			if tt.key == nil {
				assert.Equal(t, []int{0, 1, 2, 0, 1, 2}, partitions)
			} else {
				for _, partition := range partitions {
					assert.Equal(t, partitions[0], partition)
				}
			}
		})
	}
}

func TestKafkaClient_ConfiguredKeyPublishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	writer := NewMockWriter(ctrl)
	metrics := NewMockMetrics(ctrl)
	publishErr := testutil.CustomError{ErrorMessage: "publish failed"}
	client := &kafkaClient{
		writer: writer, metrics: metrics, logger: logging.NewMockLogger(logging.DEBUG),
		keyFields: keyfields.Parse("orders:orderId"),
	}
	writer.EXPECT().WriteMessages(gomock.Any(), gomock.Any()).Return(publishErr)
	metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", "orders")
	require.ErrorIs(t, client.Publish(t.Context(), "orders", []byte(`{"orderId":"123"}`)), publishErr)
}

func TestKafkaWriterConfiguredBalancer(t *testing.T) {
	for _, mapping := range []string{"", "invalid", "orders:orderId"} {
		t.Run(mapping, func(t *testing.T) {
			writer := createKafkaWriter(&Config{
				Brokers: []string{"localhost:9092"}, BatchSize: 1,
				BatchBytes: DefaultBatchBytes, BatchTimeout: DefaultBatchTimeout, KeyFields: mapping,
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
		})
	}
}

func TestNewParsesKeyMappings(t *testing.T) {
	stubConnectToBrokers(t, nil)

	ctrl := gomock.NewController(t)
	client := New(&Config{
		Brokers: []string{"localhost:9092"}, BatchSize: 1,
		BatchBytes: DefaultBatchBytes, BatchTimeout: DefaultBatchTimeout, KeyFields: "orders:orderId",
	}, logging.NewMockLogger(logging.DEBUG), NewMockMetrics(ctrl))
	require.NotNil(t, client)

	t.Cleanup(func() { require.NoError(t, client.Close()) })
	assert.Equal(t, keyfields.Fields{"orders": "orderId"}, client.keyFields)

	writer, ok := client.snapshotWriter().(*kafkago.Writer)
	require.True(t, ok)
	assert.IsType(t, &kafkago.Hash{}, writer.Balancer)
}
