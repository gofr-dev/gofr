package nats

import (
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/logging"
)

func TestNewStreamManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)

	sm := newStreamManager(mockJS, logger)

	assert.NotNil(t, sm)
	assert.Equal(t, mockJS, sm.js)
	assert.Equal(t, logger, sm.logger)
}

func TestStreamManager_CreateStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)

	sm := newStreamManager(mockJS, logger)

	ctx := t.Context()
	cfg := StreamConfig{
		Stream:   "test-stream",
		Subjects: []string{"test.subject"},
	}

	mockJS.EXPECT().CreateStream(ctx, gomock.Any()).Return(nil, nil)

	err := sm.CreateStream(ctx, &cfg)
	require.NoError(t, err)
}

func TestStreamManager_CreateStream_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)

	sm := newStreamManager(mockJS, logger)

	ctx := t.Context()
	cfg := StreamConfig{
		Stream:   "test-stream",
		Subjects: []string{"test.subject"},
	}

	expectedErr := errCreateStream
	mockJS.EXPECT().CreateStream(ctx, gomock.Any()).Return(nil, expectedErr)

	err := sm.CreateStream(ctx, &cfg)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestStreamManager_DeleteStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)

	sm := newStreamManager(mockJS, logger)

	ctx := t.Context()
	streamName := "test-stream"

	mockJS.EXPECT().DeleteStream(ctx, streamName).Return(nil)

	err := sm.DeleteStream(ctx, streamName)
	require.NoError(t, err)
}

func TestStreamManager_DeleteStream_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)

	sm := newStreamManager(mockJS, logger)

	ctx := t.Context()
	streamName := "test-stream"

	mockJS.EXPECT().DeleteStream(ctx, streamName).Return(jetstream.ErrStreamNotFound)

	err := sm.DeleteStream(ctx, streamName)
	require.NoError(t, err)
}

func TestStreamManager_DeleteStream_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)

	sm := newStreamManager(mockJS, logger)

	ctx := t.Context()
	streamName := "test-stream"

	expectedErr := errDeleteStream
	mockJS.EXPECT().DeleteStream(ctx, streamName).Return(expectedErr)

	err := sm.DeleteStream(ctx, streamName)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestStreamManager_CreateOrUpdateStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)

	sm := newStreamManager(mockJS, logger)

	ctx := t.Context()
	cfg := &jetstream.StreamConfig{
		Name:     "test-stream",
		Subjects: []string{"test.subject"},
	}

	mockStream := NewMockStream(ctrl)
	mockJS.EXPECT().CreateOrUpdateStream(ctx, *cfg).Return(mockStream, nil)

	stream, err := sm.CreateOrUpdateStream(ctx, cfg)
	require.NoError(t, err)
	assert.Equal(t, mockStream, stream)
}

func TestStreamManager_CreateOrUpdateStream_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)

	sm := newStreamManager(mockJS, logger)

	ctx := t.Context()
	cfg := &jetstream.StreamConfig{
		Name:     "test-stream",
		Subjects: []string{"test.subject"},
	}

	expectedErr := errCreateOrUpdateStream
	mockJS.EXPECT().CreateOrUpdateStream(ctx, *cfg).Return(nil, expectedErr)

	stream, err := sm.CreateOrUpdateStream(ctx, cfg)
	require.Error(t, err)
	assert.Nil(t, stream)
	assert.Equal(t, expectedErr, err)
}

func TestStreamManager_GetStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)

	sm := newStreamManager(mockJS, logger)

	ctx := t.Context()
	streamName := "test-stream"

	mockStream := NewMockStream(ctrl)
	mockJS.EXPECT().Stream(ctx, streamName).Return(mockStream, nil)

	stream, err := sm.GetStream(ctx, streamName)
	require.NoError(t, err)
	assert.Equal(t, mockStream, stream)
}

func TestStreamManager_GetStream_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)

	sm := newStreamManager(mockJS, logger)

	ctx := t.Context()
	streamName := "test-stream"

	mockJS.EXPECT().Stream(ctx, streamName).Return(nil, jetstream.ErrStreamNotFound)

	stream, err := sm.GetStream(ctx, streamName)
	require.Error(t, err)
	assert.Nil(t, stream)
	assert.Equal(t, jetstream.ErrStreamNotFound, err)
}

func TestStreamManager_GetStream_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockJS := NewMockJetStream(ctrl)
	logger := logging.NewMockLogger(logging.DEBUG)

	sm := newStreamManager(mockJS, logger)

	ctx := t.Context()
	streamName := "test-stream"

	expectedErr := errGetStream
	mockJS.EXPECT().Stream(ctx, streamName).Return(nil, expectedErr)

	stream, err := sm.GetStream(ctx, streamName)
	require.Error(t, err)
	assert.Nil(t, stream)
	assert.Equal(t, expectedErr, err)
}

func TestStreamManager_CreateStream_Config(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *StreamConfig
		expJSCfg  jetstream.StreamConfig
		createErr error
		expErr    error
	}{
		{
			name: "file storage with limits retention",
			cfg: &StreamConfig{Stream: "s", Subjects: []string{"a"}, MaxBytes: 10, MaxAge: time.Hour,
				Storage: "file", Retention: "limits"},
			expJSCfg: jetstream.StreamConfig{Name: "s", Subjects: []string{"a"}, MaxBytes: 10, MaxAge: time.Hour,
				Storage: jetstream.FileStorage, Retention: jetstream.LimitsPolicy},
		},
		{
			name: "memory storage with interest retention",
			cfg:  &StreamConfig{Stream: "s", Subjects: []string{"a"}, Storage: "memory", Retention: "interest"},
			expJSCfg: jetstream.StreamConfig{Name: "s", Subjects: []string{"a"},
				Storage: jetstream.MemoryStorage, Retention: jetstream.InterestPolicy},
		},
		{
			name:     "workqueue retention with unknown storage left at default",
			cfg:      &StreamConfig{Stream: "s", Subjects: []string{"a"}, Storage: "tape", Retention: "workqueue"},
			expJSCfg: jetstream.StreamConfig{Name: "s", Subjects: []string{"a"}, Retention: jetstream.WorkQueuePolicy},
		},
		{
			name:     "unknown retention left at default",
			cfg:      &StreamConfig{Stream: "s", Subjects: []string{"a"}, Retention: "forever"},
			expJSCfg: jetstream.StreamConfig{Name: "s", Subjects: []string{"a"}},
		},
		{
			name:      "stream already in use is treated as success",
			cfg:       &StreamConfig{Stream: "s", Subjects: []string{"a"}},
			expJSCfg:  jetstream.StreamConfig{Name: "s", Subjects: []string{"a"}},
			createErr: jetstream.ErrStreamNameAlreadyInUse,
			expErr:    nil,
		},
		{
			name:      "wrapped stream already in use is treated as success",
			cfg:       &StreamConfig{Stream: "s", Subjects: []string{"a"}},
			expJSCfg:  jetstream.StreamConfig{Name: "s", Subjects: []string{"a"}},
			createErr: fmt.Errorf("create stream: %w", jetstream.ErrStreamNameAlreadyInUse),
			expErr:    nil,
		},
		{
			name:     "stream name in use error code with reworded description is treated as success",
			cfg:      &StreamConfig{Stream: "s", Subjects: []string{"a"}},
			expJSCfg: jetstream.StreamConfig{Name: "s", Subjects: []string{"a"}},
			createErr: &jetstream.APIError{ErrorCode: jetstream.JSErrCodeStreamNameInUse,
				Description: "stream exists"},
			expErr: nil,
		},
		{
			// jetstream.APIError.Is matches by error code, so this 10059 error is reported as
			// ErrStreamNotFound even though its text mentions "stream name already in use".
			name:     "other api error mentioning stream name in use is not tolerated",
			cfg:      &StreamConfig{Stream: "s", Subjects: []string{"a"}},
			expJSCfg: jetstream.StreamConfig{Name: "s", Subjects: []string{"a"}},
			createErr: &jetstream.APIError{ErrorCode: jetstream.JSErrCodeStreamNotFound,
				Description: "stream name already in use"},
			expErr: jetstream.ErrStreamNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockJS := NewMockJetStream(ctrl)

			sm := newStreamManager(mockJS, logging.NewMockLogger(logging.DEBUG))

			mockJS.EXPECT().CreateStream(gomock.Any(), tt.expJSCfg).Return(nil, tt.createErr)

			err := sm.CreateStream(t.Context(), tt.cfg)

			require.ErrorIs(t, err, tt.expErr)
		})
	}
}
