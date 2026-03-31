package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource/file"
)

var errFileNotFound = errors.New("file not found")

func setupMockFS(ctrl *gomock.Controller, content string, openErr error) *file.MockFileSystem {
	mockFS := file.NewMockFileSystem(ctrl)

	if openErr != nil {
		mockFS.EXPECT().Open(gomock.Any()).Return(nil, openErr).AnyTimes()
		return mockFS
	}

	mockFile := file.NewMockFile(ctrl)
	mockFile.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
		reader := bytes.NewReader([]byte(content))
		n, err := reader.Read(p)
		if err != nil {
			return n, err
		}
		return n, io.EOF
	}).AnyTimes()
	mockFile.EXPECT().Close().Return(nil).AnyTimes()

	mockFS.EXPECT().Open(gomock.Any()).Return(mockFile, nil).AnyTimes()

	return mockFS
}

func TestNewFileTokenAuthConfig(t *testing.T) {
	testCases := []struct {
		name            string
		fs              func(*gomock.Controller) *file.MockFileSystem
		tokenFilePath   string
		refreshInterval time.Duration
		wantErr         bool
		errContains     string
	}{
		{
			name: "success with explicit path",
			fs: func(ctrl *gomock.Controller) *file.MockFileSystem {
				return setupMockFS(ctrl, "my-token-value", nil)
			},
			tokenFilePath:   "/custom/path/token",
			refreshInterval: time.Minute,
		},
		{
			name: "success with default path",
			fs: func(ctrl *gomock.Controller) *file.MockFileSystem {
				return setupMockFS(ctrl, "my-token-value", nil)
			},
			tokenFilePath:   "",
			refreshInterval: time.Minute,
		},
		{
			name:            "nil file system",
			fs:              nil,
			tokenFilePath:   "/some/path",
			refreshInterval: time.Minute,
			wantErr:         true,
			errContains:     "file system is required",
		},
		{
			name: "file open error",
			fs: func(ctrl *gomock.Controller) *file.MockFileSystem {
				return setupMockFS(ctrl, "", errFileNotFound)
			},
			tokenFilePath:   "/bad/path",
			refreshInterval: time.Minute,
			wantErr:         true,
			errContains:     "failed to read token",
		},
		{
			name: "empty token file",
			fs: func(ctrl *gomock.Controller) *file.MockFileSystem {
				return setupMockFS(ctrl, "   \n  ", nil)
			},
			tokenFilePath:   "/empty/token",
			refreshInterval: time.Minute,
			wantErr:         true,
			errContains:     "token file is empty",
		},
		{
			name: "zero interval defaults to 30s",
			fs: func(ctrl *gomock.Controller) *file.MockFileSystem {
				return setupMockFS(ctrl, "token-val", nil)
			},
			tokenFilePath:   "/path",
			refreshInterval: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			var fs file.FileSystem
			if tc.fs != nil {
				fs = tc.fs(ctrl)
			}

			provider, err := NewFileTokenAuthConfig(fs, tc.tokenFilePath, tc.refreshInterval)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				assert.Nil(t, provider)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, provider)

			t.Cleanup(func() {
				provider.Close()
			})
		})
	}
}

func TestFileTokenSource_Token(t *testing.T) {
	testCases := []struct {
		name      string
		token     string
		wantValue string
		wantErr   bool
	}{
		{
			name:      "valid token",
			token:     "my-jwt-token-value",
			wantValue: "my-jwt-token-value",
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			src := &fileTokenSource{
				token: tc.token,
				done:  make(chan struct{}),
			}

			value, err := src.Token(context.Background())

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantValue, value)
		})
	}
}

func TestFileTokenSource_RefreshLoop(t *testing.T) {
	testCases := []struct {
		name          string
		initialToken  string
		refreshToken  string
		refreshErr    error
		expectedToken string
	}{
		{
			name:          "updates token on refresh",
			initialToken:  "old-token",
			refreshToken:  "new-token",
			expectedToken: "new-token",
		},
		{
			name:          "keeps last good token on error",
			initialToken:  "good-token",
			refreshErr:    errFileNotFound,
			expectedToken: "good-token",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockFS := file.NewMockFileSystem(ctrl)

			initialFile := file.NewMockFile(ctrl)
			initialFile.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				n := copy(p, tc.initialToken)
				return n, io.EOF
			})
			initialFile.EXPECT().Close().Return(nil)
			mockFS.EXPECT().Open(gomock.Any()).Return(initialFile, nil)

			if tc.refreshErr != nil {
				mockFS.EXPECT().Open(gomock.Any()).Return(nil, tc.refreshErr).AnyTimes()
			} else {
				refreshFile := file.NewMockFile(ctrl)
				refreshFile.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
					n := copy(p, tc.refreshToken)
					return n, io.EOF
				}).AnyTimes()
				refreshFile.EXPECT().Close().Return(nil).AnyTimes()
				mockFS.EXPECT().Open(gomock.Any()).Return(refreshFile, nil).AnyTimes()
			}

			provider, err := NewFileTokenAuthConfig(mockFS, "/path/token", 50*time.Millisecond)
			require.NoError(t, err)

			t.Cleanup(func() { provider.Close() })

			time.Sleep(100 * time.Millisecond)

			token, err := provider.Token(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tc.expectedToken, token)
		})
	}
}

func TestFileTokenSource_Close(t *testing.T) {
	ctrl := gomock.NewController(t)

	provider, err := NewFileTokenAuthConfig(
		setupMockFS(ctrl, "test-token", nil),
		"/path/token",
		time.Minute,
	)
	require.NoError(t, err)

	err = provider.Close()
	assert.NoError(t, err)

	// Double close should not panic.
	err = provider.Close()
	assert.NoError(t, err)
}
