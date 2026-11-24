package eventhub

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

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

func TestPublishPartitionKey(t *testing.T) {
	for _, tt := range []struct {
		name     string
		key      []byte
		explicit bool
	}{
		{name: "absent"},
		{name: "explicit nil", explicit: true},
		{name: "empty", key: []byte{}, explicit: true},
		{name: "keyed", key: []byte("order-123"), explicit: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			logger, metrics := NewMockLogger(ctrl), NewMockMetrics(ctrl)
			producer := &partitionKeyProducer{}
			client := New(getTestConfigs())
			client.UseLogger(logger)
			client.UseMetrics(metrics)
			client.producer = producer

			metrics.EXPECT().IncrementCounter(gomock.Any(), "app_pubsub_publish_total_count", "topic", "event-hub-name")
			logger.EXPECT().Errorf("failed to create event batch %v", context.Canceled)

			ctx := t.Context()
			if tt.explicit {
				ctx = pubsub.WithPartitionKey(ctx, tt.key)
			}

			require.ErrorIs(t, client.Publish(ctx, "event-hub-name", []byte{0xff, 0x00, 0x81}), context.Canceled)
			require.NotNil(t, producer.options)

			if len(tt.key) == 0 {
				require.Nil(t, producer.options.PartitionKey)
			} else {
				require.NotNil(t, producer.options.PartitionKey)
				require.Equal(t, string(tt.key), *producer.options.PartitionKey)
			}
		})
	}
}
