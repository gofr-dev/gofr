package kafka

import (
	"context"
	"testing"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/datasource/pubsub/internal/keyfields"
	"gofr.dev/pkg/gofr/logging"
)

func TestPublishWithOpaquePayloadAndContextKey(t *testing.T) {
	protobuf, err := proto.Marshal(wrapperspb.String("protobuf order"))
	require.NoError(t, err)

	for _, tt := range []struct {
		name    string
		payload []byte
	}{
		{"protobuf", protobuf},
		{"arbitrary bytes", []byte{0xff, 0x00, 0x81}},
		{"JSON", []byte(`{"orderId":"ignored-json-key"}`)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			writer, metrics := NewMockWriter(ctrl), NewMockMetrics(ctrl)
			client := &kafkaClient{
				writer: writer, metrics: metrics, logger: logging.NewMockLogger(logging.DEBUG),
				config: Config{Brokers: []string{"localhost:9092"}}, keyFields: keyfields.Parse("orders:orderId"),
			}

			parent, cancel := context.WithCancel(t.Context())
			defer cancel()

			ctx := pubsub.WithPartitionKey(parent, []byte("order-123"))
			balancer := &kafkago.Hash{}
			partitions := make([]int, 0, 3)

			writer.EXPECT().WriteMessages(gomock.Any(), gomock.Any()).Times(3).DoAndReturn(
				func(publishCtx context.Context, messages ...kafkago.Message) error {
					require.Len(t, messages, 1)
					assert.Equal(t, parent.Done(), publishCtx.Done())
					assert.Equal(t, []byte("order-123"), messages[0].Key)
					assert.Equal(t, tt.payload, messages[0].Value)
					partitions = append(partitions, balancer.Balance(messages[0], 0, 1, 2))

					return nil
				},
			)
			metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", "orders").Times(3)
			metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_success_count", "topic", "orders").Times(3)

			for range 3 {
				require.NoError(t, client.Publish(ctx, "orders", tt.payload))
			}

			assert.Equal(t, []int{partitions[0], partitions[0], partitions[0]}, partitions)
		})
	}
}
