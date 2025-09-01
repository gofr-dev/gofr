package azure

import (
	"context"
	"io"
)

// MockAzureClient is a mock implementation of azureClient for testing.
type MockAzureClient struct {
	// Mock responses
	CreateDirectoryResponse any
	CreateDirectoryError    error
	DeleteDirectoryResponse any
	DeleteDirectoryError    error
	ListFilesResponse       any
	ListFilesError          error
	CreateFileResponse      any
	CreateFileError         error
	DeleteFileResponse      any
	DeleteFileError         error
	DownloadFileResponse    any
	DownloadFileError       error
	UploadRangeResponse     any
	UploadRangeError        error
	GetPropertiesResponse   any
	GetPropertiesError      error
}

func (m *MockAzureClient) CreateDirectory(_ context.Context, _ string, _ any) (any, error) {
	return m.CreateDirectoryResponse, m.CreateDirectoryError
}

func (m *MockAzureClient) DeleteDirectory(_ context.Context, _ string, _ any) (any, error) {
	return m.DeleteDirectoryResponse, m.DeleteDirectoryError
}

func (m *MockAzureClient) ListFilesAndDirectoriesSegment(_ context.Context, _ *string, _ any) (any, error) {
	return m.ListFilesResponse, m.ListFilesError
}

func (m *MockAzureClient) CreateFile(_ context.Context, _ string, _ int64, _ any) (any, error) {
	return m.CreateFileResponse, m.CreateFileError
}

func (m *MockAzureClient) DeleteFile(_ context.Context, _ string, _ any) (any, error) {
	return m.DeleteFileResponse, m.DeleteFileError
}

func (m *MockAzureClient) DownloadFile(_ context.Context, _ any) (any, error) {
	return m.DownloadFileResponse, m.DownloadFileError
}

func (m *MockAzureClient) UploadRange(_ context.Context, _ int64, _ io.ReadSeekCloser, _ any) (any, error) {
	return m.UploadRangeResponse, m.UploadRangeError
}

func (m *MockAzureClient) GetProperties(_ context.Context, _ any) (any, error) {
	return m.GetPropertiesResponse, m.GetPropertiesError
}
