package eventhub

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"go.uber.org/mock/gomock"
)

// TestMessage_Commit covers direct-read mode, where there is no processor checkpoint to update.
// The processor branch needs a *azeventhubs.ProcessorPartitionClient, which only a running
// Processor can hand out -- its zero value nil-derefs on UpdateCheckpoint.
func TestMessage_Commit(t *testing.T) {
	testCases := []struct {
		name  string
		event *azeventhubs.ReceivedEventData
	}{
		{name: "direct read with event", event: &azeventhubs.ReceivedEventData{EventData: azeventhubs.EventData{Body: []byte("m")}}},
		{name: "direct read without event", event: nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockLogger := NewMockLogger(gomock.NewController(t))
			mockLogger.EXPECT().Debugf("Message acknowledged (direct read mode)")

			msg := &Message{event: tc.event, logger: mockLogger}

			msg.Commit()
		})
	}
}
