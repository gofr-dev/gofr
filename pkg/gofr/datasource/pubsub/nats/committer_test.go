package nats

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNATSCommitter_Commit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMsg := NewMockMsg(ctrl)
	committer := createTestCommitter(mockMsg)

	t.Run("Successful Commit", func(_ *testing.T) {
		mockMsg.EXPECT().Ack().Return(nil)

		committer.Commit()
	})

	t.Run("Failed Commit with Successful Nak", func(_ *testing.T) {
		mockMsg.EXPECT().Ack().Return(assert.AnError)
		mockMsg.EXPECT().Nak().Return(nil)

		committer.Commit()
	})

	t.Run("Failed Commit with Failed Nak", func(_ *testing.T) {
		mockMsg.EXPECT().Ack().Return(assert.AnError)
		mockMsg.EXPECT().Nak().Return(assert.AnError)

		committer.Commit()
	})
}

func TestNATSCommitter_Nak(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMsg := NewMockMsg(ctrl)
	committer := createTestCommitter(mockMsg)

	t.Run("Successful Nak", func(t *testing.T) {
		mockMsg.EXPECT().Nak().Return(nil)

		err := committer.Nak()
		assert.NoError(t, err)
	})

	t.Run("Failed Nak", func(t *testing.T) {
		mockMsg.EXPECT().Nak().Return(assert.AnError)

		err := committer.Nak()
		assert.Error(t, err)
	})
}

func TestNATSCommitter_Rollback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMsg := NewMockMsg(ctrl)
	committer := createTestCommitter(mockMsg)

	t.Run("Successful Rollback", func(t *testing.T) {
		mockMsg.EXPECT().Nak().Return(nil)

		err := committer.Rollback()
		assert.NoError(t, err)
	})

	t.Run("Failed Rollback", func(t *testing.T) {
		mockMsg.EXPECT().Nak().Return(assert.AnError)

		err := committer.Rollback()
		assert.Error(t, err)
	})
}
