package eventhub

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
)

// parseQueryArgs parses the query arguments.
func (c *Client) parseQueryArgs(args ...any) (startPosition azeventhubs.StartPosition, limit int) {
	// Default to earliest
	earliest := true
	startPosition = azeventhubs.StartPosition{
		Earliest: &earliest,
	}
	limit = 10

	if len(args) > 0 {
		switch v := args[0].(type) {
		case int64:
			if v > 0 {
				startPosition = azeventhubs.StartPosition{
					SequenceNumber: &v,
					Inclusive:      true,
				}
			}
		case string:
			if v == "latest" {
				latest := true
				startPosition = azeventhubs.StartPosition{
					Latest: &latest,
				}
			}
		case time.Time:
			startPosition = azeventhubs.StartPosition{
				EnqueuedTime: &v,
			}
		}
	}

	if len(args) > 1 {
		if val, ok := args[1].(int); ok && val > 0 {
			limit = val
		}
	}

	return startPosition, limit
}

// readMessages reads messages from Event Hub partitions.
func (c *Client) readMessages(ctx context.Context, startPosition azeventhubs.StartPosition, limit int) ([]byte, error) {
	partitions, err := c.consumer.GetEventHubProperties(ctx, nil)
	if err != nil {
		return nil, err
	}

	var result []byte

	messagesRead := 0

	// Read from partitions sequentially until we get enough messages
	for _, partitionID := range partitions.PartitionIDs {
		if messagesRead >= limit {
			break
		}

		remaining := limit - messagesRead
		messages := c.readFromPartition(ctx, partitionID, startPosition, remaining)

		for _, msg := range messages {
			if len(result) > 0 {
				result = append(result, '\n')
			}

			result = append(result, msg...)

			messagesRead++

			if messagesRead >= limit {
				break
			}
		}
	}

	return result, nil
}

// readFromPartition reads messages from a single partition.
func (c *Client) readFromPartition(ctx context.Context, partitionID string, startPosition azeventhubs.StartPosition, maxMessages int) [][]byte {
	partitionClient, err := c.consumer.NewPartitionClient(partitionID, &azeventhubs.PartitionClientOptions{
		StartPosition: startPosition,
	})
	if err != nil {
		return nil
	}
	defer partitionClient.Close(ctx)

	var messages [][]byte

	for len(messages) < maxMessages {
		select {
		case <-ctx.Done():
			return messages
		default:
			events, err := partitionClient.ReceiveEvents(ctx, maxMessages-len(messages), nil)
			if err != nil || len(events) == 0 {
				return messages
			}

			for _, event := range events {
				messages = append(messages, event.Body)
				if len(messages) >= maxMessages {
					break
				}
			}
		}
	}

	return messages
}
