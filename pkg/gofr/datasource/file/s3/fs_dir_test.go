package s3

import (
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_Mkdir_PutObjectFails_Error(t *testing.T) {
	ctrl := gomock.NewController(t)

	mocks := setupTestMocks(ctrl)
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Debug(gomock.Any())
	mocks.mockS3.EXPECT().PutObject(gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{}, nil)
	mocks.mockS3.EXPECT().PutObject(gomock.Any(), gomock.Any()).Return(nil, errMock)

	err := fs.Mkdir("abc/def/ghi", os.ModePerm)

	require.ErrorIs(t, err, errMock)
}

func Test_RemoveAll(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		setupMock func(m *testMocks)
		expErr    error
	}{
		{
			name: "path with extension removes single file",
			path: "abc/file.txt",
			setupMock: func(m *testMocks) {
				m.mockS3.EXPECT().DeleteObject(gomock.Any(), gomock.Any()).Return(&s3.DeleteObjectOutput{}, nil)
				m.mockLogger.EXPECT().Logf("File with path %q deleted", "abc/file.txt")
			},
			expErr: nil,
		},
		{
			name: "listing objects fails",
			path: "abc",
			setupMock: func(m *testMocks) {
				m.mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(nil, errMock)
			},
			expErr: errMock,
		},
		{
			name: "deleting objects fails",
			path: "abc",
			setupMock: func(m *testMocks) {
				m.mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(&s3.ListObjectsV2Output{
					Contents: []types.Object{{Key: aws.String("abc/file.txt")}},
				}, nil)
				m.mockS3.EXPECT().DeleteObjects(gomock.Any(), gomock.Any()).Return(nil, errMock)
				m.mockLogger.EXPECT().Errorf("Error while deleting directory: %v", errMock)
			},
			expErr: errMock,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := setupTestMocks(gomock.NewController(t))
			fs := setupTestFileSystem(mocks, nil)

			mocks.mockLogger.EXPECT().Debug(gomock.Any())
			tt.setupMock(mocks)

			err := fs.RemoveAll(tt.path)

			require.ErrorIs(t, err, tt.expErr)
		})
	}
}

func Test_ReadDir_ListObjectsV2Fails_Error(t *testing.T) {
	mocks := setupTestMocks(gomock.NewController(t))
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Debug(gomock.Any())
	mocks.mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(nil, errMock)

	entries, err := fs.ReadDir("abc")

	require.ErrorIs(t, err, errMock)
	assert.Nil(t, entries)
}

func Test_ChDir_NotPermitted(t *testing.T) {
	mocks := setupTestMocks(gomock.NewController(t))
	fs := setupTestFileSystem(mocks, nil)

	logs := make([]FileLog, 0)

	mocks.mockLogger.EXPECT().Debug(gomock.Any()).Do(func(args ...any) {
		logs = append(logs, *args[0].(*FileLog))
	})

	err := fs.ChDir("abc")

	require.ErrorIs(t, err, ErrOperationNotPermitted)
	require.Len(t, logs, 1)
	assert.Equal(t, "CHDIR", logs[0].Operation)
	assert.Equal(t, statusErr, *logs[0].Status)
}

func Test_Getwd(t *testing.T) {
	mocks := setupTestMocks(gomock.NewController(t))
	fs := setupTestFileSystem(mocks, nil)

	mocks.mockLogger.EXPECT().Debug(gomock.Any())

	wd, err := fs.Getwd()

	require.NoError(t, err)
	assert.Equal(t, "/test-bucket", wd)
}

func Test_RenameDirectory_Errors(t *testing.T) {
	listOutput := &s3.ListObjectsV2Output{Contents: []types.Object{{Key: aws.String("old-dir/file.txt")}}}

	tests := []struct {
		name      string
		setupMock func(m *testMocks)
		expErr    error
	}{
		{
			name: "copying objects fails",
			setupMock: func(m *testMocks) {
				m.mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(listOutput, nil)
				m.mockS3.EXPECT().CopyObject(gomock.Any(), gomock.Any()).Return(nil, errMock)
			},
			expErr: errMock,
		},
		{
			name: "removing old directory fails",
			setupMock: func(m *testMocks) {
				m.mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(listOutput, nil)
				m.mockS3.EXPECT().CopyObject(gomock.Any(), gomock.Any()).Return(&s3.CopyObjectOutput{}, nil)
				m.mockS3.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).Return(nil, errMock)
				m.mockLogger.EXPECT().Debug(gomock.Any())
			},
			expErr: errMock,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := setupTestMocks(gomock.NewController(t))
			fs := setupTestFileSystem(mocks, nil)

			mocks.mockLogger.EXPECT().Debug(gomock.Any())
			tt.setupMock(mocks)

			err := fs.Rename("old-dir", "new-dir")

			require.ErrorIs(t, err, tt.expErr)
		})
	}
}

func Test_Stat_ZeroPrefixedBinaryFile(t *testing.T) {
	modified := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	mocks := setupTestMocks(gomock.NewController(t))
	fs := setupTestFileSystem(mocks, nil)

	// Stat and the FileInfo accessors each emit an operation log.
	mocks.mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocks.mockS3.EXPECT().ListObjectsV2(gomock.Any(), &s3.ListObjectsV2Input{
		Bucket: aws.String("test-bucket"),
		Prefix: aws.String("binary"),
	}).Return(&s3.ListObjectsV2Output{
		Contents: []types.Object{{Key: aws.String("binary"), Size: aws.Int64(42), LastModified: aws.Time(modified)}},
	}, nil)

	info, err := fs.Stat("0binary")

	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, int64(42), info.Size())
	assert.Equal(t, modified, info.ModTime())
	assert.False(t, info.IsDir())
}
