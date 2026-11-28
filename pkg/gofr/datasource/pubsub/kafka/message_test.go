package kafka

import (
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestNewMessage(t *testing.T) {
	msg := new(kafka.Message)
	reader := new(kafka.Reader)
	k := newKafkaMessage(msg, reader, nil)

	assert.NotNil(t, k)
	assert.Equal(t, msg, k.msg)
	assert.Equal(t, reader, k.reader)
}

func TestKafkaMessage_Commit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReader := NewMockReader(ctrl)

	msg := &kafka.Message{Topic: "test", Value: []byte("hello")}
	logger := logging.NewMockLogger(logging.ERROR)
	k := newKafkaMessage(msg, mockReader, logger)

	mockReader.EXPECT().CommitMessages(gomock.Any(), *msg).Return(nil)

	k.Commit()
}

func TestKafkaMessage_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReader := NewMockReader(ctrl)

	out := testutil.StderrOutputForFunc(func() {
		msg := &kafka.Message{Topic: "test", Value: []byte("hello")}
		logger := logging.NewMockLogger(logging.ERROR)
		k := newKafkaMessage(msg, mockReader, logger)

		mockReader.EXPECT().CommitMessages(gomock.Any(), *msg).
			Return(testutil.CustomError{ErrorMessage: "error"})

		k.Commit()
	})

	assert.Contains(t, out, "unable to commit message on kafka")
}

func TestKafkaMessage_Redeliver(t *testing.T) {
	errCommit := testutil.CustomError{ErrorMessage: "commit failed"}

	tests := []struct {
		desc      string
		msg       kafka.Message
		expRewind kafka.Message
		commitErr error
		expLog    string
	}{
		{
			desc:      "group position is set back to the failed message",
			msg:       kafka.Message{Topic: "orders", Partition: 2, Offset: 5},
			expRewind: kafka.Message{Topic: "orders", Partition: 2, Offset: 4},
		},
		{
			desc:      "first message of a partition",
			msg:       kafka.Message{Topic: "orders", Partition: 0, Offset: 0},
			expRewind: kafka.Message{Topic: "orders", Partition: 0, Offset: -1},
		},
		{
			desc:      "commit failure is logged and the reader is still released",
			msg:       kafka.Message{Topic: "orders", Partition: 1, Offset: 9},
			expRewind: kafka.Message{Topic: "orders", Partition: 1, Offset: 8},
			commitErr: errCommit,
			expLog:    "unable to reset the kafka offset to redeliver the message: commit failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			mockReader := NewMockReader(gomock.NewController(t))
			mockReader.EXPECT().CommitMessages(gomock.Any(), tc.expRewind).Return(tc.commitErr)

			var released Reader

			out := testutil.StderrOutputForFunc(func() {
				k := newKafkaMessage(&tc.msg, mockReader, logging.NewMockLogger(logging.ERROR))
				k.release = func(r Reader) { released = r }

				k.Redeliver()
			})

			assert.Equal(t, mockReader, released)
			assert.Contains(t, out, tc.expLog)
		})
	}
}

func TestKafkaMessage_Redeliver_WithoutReader(t *testing.T) {
	released := false

	k := newKafkaMessage(&kafka.Message{Topic: "orders"}, nil, nil)
	k.release = func(Reader) { released = true }

	k.Redeliver()

	assert.False(t, released)
}
