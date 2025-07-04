package eventhub

import (
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/stretchr/testify/require"
)

func TestParseQueryArgs(t *testing.T) {
	client := New(Config{})
	now := time.Now()

	tests := []struct {
		name          string
		args          []any
		expectedStart azeventhubs.StartPosition
		expectedLimit int
	}{
		{
			name:          "Default values",
			args:          nil,
			expectedStart: azeventhubs.StartPosition{Earliest: boolPtr(true)},
			expectedLimit: 10,
		},
		{
			name:          "With sequence number",
			args:          []any{int64(5)},
			expectedStart: azeventhubs.StartPosition{SequenceNumber: int64Ptr(5), Inclusive: true},
			expectedLimit: 10,
		},
		{
			name:          "With latest",
			args:          []any{"latest"},
			expectedStart: azeventhubs.StartPosition{Latest: boolPtr(true)},
			expectedLimit: 10,
		},
		{
			name:          "With enqueued time",
			args:          []any{now},
			expectedStart: azeventhubs.StartPosition{EnqueuedTime: &now},
			expectedLimit: 10,
		},
		{
			name:          "With limit",
			args:          []any{int64(5), 20},
			expectedStart: azeventhubs.StartPosition{SequenceNumber: int64Ptr(5), Inclusive: true},
			expectedLimit: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startPosition, limit := client.parseQueryArgs(tt.args...)
			require.Equal(t, tt.expectedStart, startPosition, "Start position mismatch")
			require.Equal(t, tt.expectedLimit, limit, "Limit mismatch")
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}
