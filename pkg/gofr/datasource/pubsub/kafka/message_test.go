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
