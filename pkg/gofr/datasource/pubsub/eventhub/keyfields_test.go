package eventhub

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

type partitionKeyProducer struct {
	options *azeventhubs.EventDataBatchOptions
}

func (p *partitionKeyProducer) NewEventDataBatch(
	_ context.Context, options *azeventhubs.EventDataBatchOptions,
) (*azeventhubs.EventDataBatch, error) {
	p.options = options

	return nil, context.Canceled
}

func (*partitionKeyProducer) SendEventDataBatch(
	context.Context, *azeventhubs.EventDataBatch, *azeventhubs.SendEventDataBatchOptions,
) error {
	return nil
}

func (*partitionKeyProducer) Close(context.Context) error { return nil }

func TestPublishConfiguredPartitionKey(t *testing.T) {
	for _, tt := range []struct {
		name, mapping, payload, expected string
	}{
		{"configured", "event-hub-name:orderId", `{"orderId":"order-123"}`, "order-123"},
		{"other topic", "orders:orderId", `{"orderId":"order-123"}`, ""},
		{"no config", "", `{"orderId":"order-123"}`, ""},
		{"missing", "event-hub-name:orderId", `{}`, ""},
		{"invalid JSON", "event-hub-name:orderId", `payload`, ""},
		{"empty", "event-hub-name:orderId", `{"orderId":""}`, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			logger, metrics := NewMockLogger(ctrl), NewMockMetrics(ctrl)
			producer := &partitionKeyProducer{}
			client := New(getTestConfigs())
			client.UseLogger(logger)
			client.UseMetrics(metrics)
			client.UseConfig(config.NewMockConfig(map[string]string{"PUBSUB_KEY_FIELDS": tt.mapping}))
			client.producer = producer

			metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", "event-hub-name")
			logger.EXPECT().Errorf("failed to create event batch %v", context.Canceled)

			switch tt.name {
			case "missing", "empty":
				logger.EXPECT().Debugf(gomock.Any(), "event-hub-name", "orderId")
			case "invalid JSON":
				logger.EXPECT().Debugf(gomock.Any(), "event-hub-name")
			}

			require.ErrorIs(t, client.Publish(t.Context(), "event-hub-name", []byte(tt.payload)), context.Canceled)
			require.NotNil(t, producer.options)

			if tt.expected == "" {
				require.Nil(t, producer.options.PartitionKey)
			} else {
				require.NotNil(t, producer.options.PartitionKey)
				require.Equal(t, tt.expected, *producer.options.PartitionKey)
			}
		})
	}
}

func TestUseConfigUnsupportedValue(t *testing.T) {
	client := New(getTestConfigs())
	client.UseConfig(nil)
	require.Empty(t, client.keyFields)
}

func TestPublishExplicitPartitionKey(t *testing.T) {
	for _, key := range [][]byte{nil, {}, []byte("order-123")} {
		t.Run(string(key), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			logger, metrics := NewMockLogger(ctrl), NewMockMetrics(ctrl)
			producer := &partitionKeyProducer{}
			client := New(getTestConfigs())
			client.UseLogger(logger)
			client.UseMetrics(metrics)
			client.UseConfig(config.NewMockConfig(map[string]string{"PUBSUB_KEY_FIELDS": "event-hub-name:orderId"}))
			client.producer = producer

			metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", "event-hub-name")
			logger.EXPECT().Errorf("failed to create event batch %v", context.Canceled)

			ctx := pubsub.WithPartitionKey(t.Context(), key)
			payload := []byte{0xff, 0x00, 0x81}
			require.ErrorIs(t, client.Publish(ctx, "event-hub-name", payload), context.Canceled)
			require.NotNil(t, producer.options)

			if len(key) == 0 {
				require.Nil(t, producer.options.PartitionKey)
			} else {
				require.NotNil(t, producer.options.PartitionKey)
				require.Equal(t, string(key), *producer.options.PartitionKey)
			}
		})
	}
}
