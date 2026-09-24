package eventhub

import (
	"context"
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
		{
			name:          "Non-positive sequence number falls back to earliest",
			args:          []any{int64(0)},
			expectedStart: azeventhubs.StartPosition{Earliest: boolPtr(true)},
			expectedLimit: 10,
		},
		{
			name:          "Unknown string falls back to earliest",
			args:          []any{"oldest"},
			expectedStart: azeventhubs.StartPosition{Earliest: boolPtr(true)},
			expectedLimit: 10,
		},
		{
			name:          "Unsupported start type falls back to earliest",
			args:          []any{3.5},
			expectedStart: azeventhubs.StartPosition{Earliest: boolPtr(true)},
			expectedLimit: 10,
		},
		{
			name:          "Non-positive limit keeps default",
			args:          []any{"latest", -1},
			expectedStart: azeventhubs.StartPosition{Latest: boolPtr(true)},
			expectedLimit: 10,
		},
		{
			name:          "Non-int limit keeps default",
			args:          []any{"latest", "20"},
			expectedStart: azeventhubs.StartPosition{Latest: boolPtr(true)},
			expectedLimit: 10,
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

func TestReadMessages(t *testing.T) {
	testCases := []struct {
		name          string
		partitions    []string
		limit         int
		expectedCalls []partitionCall
	}{
		{
			name:          "limit already reached opens no partition",
			partitions:    []string{"0", "1"},
			limit:         0,
			expectedCalls: nil,
		},
		{
			name:       "unreadable partitions are skipped",
			partitions: []string{"0", "1"},
			limit:      3,
			expectedCalls: []partitionCall{
				{partitionID: "0", startPosition: defaultStartPosition()},
				{partitionID: "1", startPosition: defaultStartPosition()},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			consumer := &mockConsumerClient{getPropsFunc: propsFunc(tc.partitions, nil)}

			client := New(Config{})
			client.consumer = consumer

			result, err := client.readMessages(t.Context(), defaultStartPosition(), tc.limit)

			require.NoError(t, err)
			require.Nil(t, result)
			require.Equal(t, tc.expectedCalls, consumer.partitionCalls)
		})
	}
}

// TestReceiveMessages covers the loop guards, which stop before the partition client is touched --
// so a nil client is safe here, and proves it is not used.
func TestReceiveMessages(t *testing.T) {
	canceled, cancel := context.WithCancel(t.Context())
	cancel()

	testCases := []struct {
		name        string
		ctx         context.Context
		maxMessages int
	}{
		{name: "context already canceled", ctx: canceled, maxMessages: 5},
		{name: "nothing requested", ctx: t.Context(), maxMessages: 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Nil(t, receiveMessages(tc.ctx, nil, tc.maxMessages))
		})
	}
}

func TestAppendMessages(t *testing.T) {
	events := []*azeventhubs.ReceivedEventData{
		{EventData: azeventhubs.EventData{Body: []byte("a")}},
		{EventData: azeventhubs.EventData{Body: []byte("b")}},
		{EventData: azeventhubs.EventData{Body: []byte("c")}},
	}

	testCases := []struct {
		name        string
		existing    [][]byte
		events      []*azeventhubs.ReceivedEventData
		maxMessages int
		expected    [][]byte
	}{
		{
			name:        "all events fit",
			events:      events,
			maxMessages: 5,
			expected:    [][]byte{[]byte("a"), []byte("b"), []byte("c")},
		},
		{
			name:        "stops at the limit",
			events:      events,
			maxMessages: 2,
			expected:    [][]byte{[]byte("a"), []byte("b")},
		},
		{
			name:        "counts messages already collected",
			existing:    [][]byte{[]byte("x")},
			events:      events,
			maxMessages: 2,
			expected:    [][]byte{[]byte("x"), []byte("a")},
		},
		{
			name:        "no events",
			existing:    [][]byte{[]byte("x")},
			maxMessages: 2,
			expected:    [][]byte{[]byte("x")},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, appendMessages(tc.existing, tc.events, tc.maxMessages))
		})
	}
}
