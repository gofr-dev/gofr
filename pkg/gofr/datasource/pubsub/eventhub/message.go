package eventhub

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
)

type Message struct {
	event     *azeventhubs.ReceivedEventData
	processor *azeventhubs.ProcessorPartitionClient
	logger    Logger
}

func (a *Message) Commit() {
	// Update the checkpoint with the latest event received
	if a.processor != nil {
		err := a.processor.UpdateCheckpoint(context.Background(), a.event, nil)
		if err != nil {
			a.logger.Errorf("failed to acknowledge event with eventID %v: %v", a.event.MessageID, err)
			return
		}

		a.logger.Debugf("Message committed via processor checkpoint (MessageID: %v)", a.event.MessageID)
	} else {
		a.logger.Debugf("Message acknowledged (direct read mode)")
	}
}
