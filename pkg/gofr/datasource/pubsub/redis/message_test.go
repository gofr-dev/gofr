package redis

import (
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewRedisMessage(t *testing.T) {
	tests := []struct {
		name     string
		msg      *redis.Message
		logger   Logger
		validate func(t *testing.T, m *Message)
	}{
		{
			name: "create message with logger",
			msg: &redis.Message{
				Channel: "test-topic",
				Payload: "test payload",
			},
			logger: func() Logger {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				return NewMockLogger(ctrl)
			}(),
			validate: func(t *testing.T, m *Message) {
				t.Helper()
				require.NotNil(t, m)
				assert.NotNil(t, m.msg)
				assert.NotNil(t, m.logger)
				assert.Equal(t, "test-topic", m.msg.Channel)
				assert.Equal(t, "test payload", m.msg.Payload)
			},
		},
		{
			name:   "create message without logger",
			msg:    &redis.Message{Channel: "test", Payload: "data"},
			logger: nil,
			validate: func(t *testing.T, m *Message) {
				t.Helper()
				require.NotNil(t, m)
				assert.NotNil(t, m.msg)
				assert.Nil(t, m.logger)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newRedisMessage(tt.msg, tt.logger)
			tt.validate(t, m)
		})
	}
}

func TestCommit(t *testing.T) {
	tests := []struct {
		name     string
		setupMsg func(t *testing.T) *Message
		validate func(t *testing.T, msg *Message)
	}{
		{
			name: "commit with logger",
			setupMsg: func(t *testing.T) *Message {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLogger := NewMockLogger(ctrl)
				mockLogger.EXPECT().Debugf("Message acknowledged (Redis PubSub doesn't require explicit acknowledgment)")

				return &Message{
					msg:    &redis.Message{Channel: "test", Payload: "data"},
					logger: mockLogger,
				}
			},
			validate: func(_ *testing.T, msg *Message) {
				// Commit should not panic
				msg.Commit()
			},
		},
		{
			name: "commit without logger",
			setupMsg: func(_ *testing.T) *Message {
				return &Message{
					msg:    &redis.Message{Channel: "test", Payload: "data"},
					logger: nil,
				}
			},
			validate: func(_ *testing.T, msg *Message) {
				// Commit should not panic even without logger
				msg.Commit()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.setupMsg(t)
			tt.validate(t, msg)
		})
	}
}
