package eventhub

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"go.uber.org/mock/gomock"
)

// TestMessage_Commit covers direct-read mode, where there is no processor checkpoint to update:
// Commit must take the acknowledgement branch, and the strict mock logger fails the test if the
// processor branch (Errorf or the checkpoint Debugf) is taken instead.
// The processor branch needs a *azeventhubs.ProcessorPartitionClient, which only a running
// Processor can hand out -- its zero value nil-derefs on UpdateCheckpoint.
func TestMessage_Commit(t *testing.T) {
	mockLogger := NewMockLogger(gomock.NewController(t))
	mockLogger.EXPECT().Debugf("Message acknowledged (direct read mode)").Times(1)

	msg := &Message{
		event:  &azeventhubs.ReceivedEventData{EventData: azeventhubs.EventData{Body: []byte("m")}},
		logger: mockLogger,
	}

	msg.Commit()
}
