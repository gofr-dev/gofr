package nats

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestNewNATSMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMsg := NewMockMsg(ctrl)
	logger := logging.NewMockLogger(logging.ERROR)
	n := newNATSMessage(mockMsg, logger)

	assert.NotNil(t, n)
	assert.Equal(t, mockMsg, n.msg)
	assert.Equal(t, logger, n.logger)
}

func TestNATSMessage_Commit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMsg := NewMockMsg(ctrl)
	logger := logging.NewMockLogger(logging.ERROR)
	n := newNATSMessage(mockMsg, logger)

	mockMsg.EXPECT().Ack().Return(nil)

	n.Commit()
}

func TestNATSMessage_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMsg := NewMockMsg(ctrl)

	out := testutil.StderrOutputForFunc(func() {
		logger := logging.NewMockLogger(logging.ERROR)
		n := newNATSMessage(mockMsg, logger)

		mockMsg.EXPECT().Ack().Return(testutil.CustomError{ErrorMessage: "ack error"})

		n.Commit()
	})

	assert.Contains(t, out, "unable to acknowledge message on Client jStream")
}
