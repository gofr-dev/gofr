package redis

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
)

func TestPubSubMessage_Commit(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "pubsub-commit-topic"

	msgChan := make(chan *pubsub.Message)
	errChan := make(chan error)

	go func() {
		msg, err := client.PubSub.Subscribe(ctx, topic)
		if err != nil {
			errChan <- err
			return
		}

		msgChan <- msg
	}()

	time.Sleep(100 * time.Millisecond)

	err := client.PubSub.Publish(ctx, topic, []byte("test"))
	require.NoError(t, err)

	select {
	case err := <-errChan:
		require.NoError(t, err)
	case msg := <-msgChan:
		require.NotNil(t, msg)
		require.NotNil(t, msg.Committer)

		assert.NotPanics(t, func() {
			msg.Committer.Commit()
		})
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestNewStreamMessage(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockLogger := logging.NewMockLogger(logging.DEBUG)
	db, _ := redismock.NewClientMock()

	t.Cleanup(func() { _ = db.Close() })

	tests := []struct {
		name    string
		client  *redis.Client
		stream  string
		group   string
		id      string
		logger  logging.Logger
		wantNil bool
		wantErr bool
	}{
		{
			name:    "valid stream message",
			client:  db,
			stream:  "test-stream",
			group:   "test-group",
			id:      "123-0",
			logger:  mockLogger,
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "empty stream name",
			client:  db,
			stream:  "",
			group:   "test-group",
			id:      "123-0",
			logger:  mockLogger,
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "empty group name",
			client:  db,
			stream:  "test-stream",
			group:   "",
			id:      "123-0",
			logger:  mockLogger,
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "empty message id",
			client:  db,
			stream:  "test-stream",
			group:   "test-group",
			id:      "",
			logger:  mockLogger,
			wantNil: false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := newStreamMessage(tt.client, tt.stream, tt.group, tt.id, tt.logger)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.client, result.client)
				assert.Equal(t, tt.stream, result.stream)
				assert.Equal(t, tt.group, result.group)
				assert.Equal(t, tt.id, result.id)
				assert.Equal(t, tt.logger, result.logger)
			}
		})
	}
}

func TestStreamMessage_Commit_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	db, mock := redismock.NewClientMock()

	t.Cleanup(func() { _ = db.Close() })

	stream := "test-stream"
	group := "test-group"
	id := "123-0"

	mock.ExpectXAck(stream, group, id).SetVal(1)

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": group,
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()

	err := client.PubSub.CreateTopic(ctx, stream)
	require.NoError(t, err)

	go func() {
		_ = client.PubSub.Publish(ctx, stream, []byte("test"))
	}()

	msg, err := client.PubSub.Subscribe(ctx, stream)
	require.NoError(t, err)
	require.NotNil(t, msg)
	require.NotNil(t, msg.Committer)

	assert.NotPanics(t, func() {
		msg.Committer.Commit()
	})
}

func TestStreamMessage_Commit_Error(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	stream := "test-stream-error"

	err := client.PubSub.CreateTopic(ctx, stream)
	require.NoError(t, err)

	go func() {
		_ = client.PubSub.Publish(ctx, stream, []byte("test"))
	}()

	msg, err := client.PubSub.Subscribe(ctx, stream)
	require.NoError(t, err)
	require.NotNil(t, msg)
	require.NotNil(t, msg.Committer)

	s.Close()

	assert.NotPanics(t, func() {
		msg.Committer.Commit()
	})
}

func TestStreamMessage_Commit_Timeout(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	stream := "test-stream-timeout"

	err := client.PubSub.CreateTopic(ctx, stream)
	require.NoError(t, err)

	go func() {
		_ = client.PubSub.Publish(ctx, stream, []byte("test"))
	}()

	msg, err := client.PubSub.Subscribe(ctx, stream)
	require.NoError(t, err)
	require.NotNil(t, msg)
	require.NotNil(t, msg.Committer)

	assert.NotPanics(t, func() {
		msg.Committer.Commit()
	})
}
