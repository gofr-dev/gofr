package azure

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	fileSystem "gofr.dev/pkg/gofr/datasource/file"
)

// File represents a file in Azure File Storage.
type File struct {
	conn         azureClient
	name         string
	offset       int64
	logger       Logger
	metrics      Metrics
	size         int64
	body         io.ReadCloser
	lastModified time.Time
	ctx          context.Context
}

var (
	ErrNilResponse = errors.New("response retrieved is nil")
)

// Close closes the response body returned in Open/Create methods if the response body is not nil.
func (f *File) Close() error {
	shareName := getShareName(f.name)

	defer f.sendOperationStats(&FileLog{
		Operation: "CLOSE",
		Location:  getLocation(shareName)}, time.Now())

	if f.body != nil {
		return f.body.Close()
	}

	return nil
}

// Read reads data into the provided byte slice.
func (f *File) Read(p []byte) (n int, err error) {
	shareName := getShareName(f.name)

	defer f.sendOperationStats(&FileLog{
		Operation: "READ",
		Location:  getLocation(shareName),
		Status:    nil,
		Message:   nil,
	}, time.Now())

	if f.conn == nil {
		return 0, ErrShareClientNotInitialized
	}

	// Download file content from Azure
	downloadResult, err := f.conn.DownloadFile(f.ctx, map[string]any{
		"path":   f.name,
		"offset": f.offset,
		"length": len(p),
	})
	if err != nil {
		// If the file doesn't exist or has no content, return EOF
		if strings.Contains(err.Error(), "ResourceNotFound") || strings.Contains(err.Error(), "InvalidRange") {
			return 0, io.EOF
		}
		return 0, fmt.Errorf("failed to download file %s: %w", f.name, err)
	}

	// Extract the response body
	response, ok := downloadResult.(map[string]any)
	if !ok {
		return 0, fmt.Errorf("invalid download response format")
	}

	body, ok := response["body"].(io.ReadCloser)
	if !ok {
		return 0, fmt.Errorf("invalid response body")
	}

	// Read data into the provided buffer
	n, err = body.Read(p)
	if err != nil && err != io.EOF {
		body.Close()
		return n, fmt.Errorf("failed to read from response body: %w", err)
	}

	// Update offset
	f.offset += int64(n)

	// Store body for potential reuse
	f.body = body

	return n, err
}

// ReadAt reads data into the provided byte slice starting at the specified offset.
func (f *File) ReadAt(p []byte, off int64) (n int, err error) {
	f.offset = off
	return f.Read(p)
}

// Seek sets the offset for the next Read or Write on file to offset.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.offset = offset
	case io.SeekCurrent:
		f.offset += offset
	case io.SeekEnd:
		f.offset = f.size + offset
	default:
		return 0, ErrInvalidWhence
	}

	return f.offset, nil
}

// Write writes data from the provided byte slice to the file.
func (f *File) Write(p []byte) (n int, err error) {
	shareName := getShareName(f.name)

	defer f.sendOperationStats(&FileLog{
		Operation: "WRITE",
		Location:  getLocation(shareName),
		Status:    nil,
		Message:   nil,
	}, time.Now())

	if f.conn == nil {
		return 0, ErrShareClientNotInitialized
	}

	// For the first write (offset 0), we need to create the file with content
	if f.offset == 0 {
		// Create a reader from the byte slice
		reader := &readSeekCloser{strings.NewReader(string(p))}

		// Upload the data range to Azure starting at offset 0
		_, err = f.conn.UploadRange(f.ctx, 0, reader, map[string]any{
			"path": f.name,
		})
		if err != nil {
			return 0, fmt.Errorf("failed to upload range to file %s: %w", f.name, err)
		}
	} else {
		// For subsequent writes, append to existing content
		reader := &readSeekCloser{strings.NewReader(string(p))}

		// Upload the data range to Azure at the current offset
		_, err = f.conn.UploadRange(f.ctx, f.offset, reader, map[string]any{
			"path": f.name,
		})
		if err != nil {
			return 0, fmt.Errorf("failed to upload range to file %s: %w", f.name, err)
		}
	}

	// Update offset and size
	f.offset += int64(len(p))
	if f.offset > f.size {
		f.size = f.offset
	}

	return len(p), nil
}

// WriteAt writes data from the provided byte slice to the file starting at the specified offset.
func (f *File) WriteAt(p []byte, off int64) (n int, err error) {
	f.offset = off
	return f.Write(p)
}

// RowReader interface for reading rows from files.
type RowReader interface {
	Next() bool
	Scan(any) error
}

// ReadAll reads all data from the file and returns a RowReader.
func (f *File) ReadAll() (fileSystem.RowReader, error) {
	// Implementation for RowReader would go here
	// For now, returning a simple implementation
	return &azureRowReader{
		file: f,
		read: false,
	}, nil
}

// GetProperties retrieves file properties from Azure
func (f *File) GetProperties() (map[string]any, error) {
	if f.conn == nil {
		return nil, ErrShareClientNotInitialized
	}

	result, err := f.conn.GetProperties(f.ctx, map[string]any{
		"path": f.name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get properties for file %s: %w", f.name, err)
	}

	props, ok := result.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid properties response format")
	}

	return props, nil
}

// readSeekCloser wraps strings.Reader to implement io.ReadSeekCloser
type readSeekCloser struct {
	*strings.Reader
}

func (r *readSeekCloser) Close() error {
	// strings.Reader doesn't need to be closed
	return nil
}

// azureRowReader implements the RowReader interface for Azure files.
type azureRowReader struct {
	file *File
	read bool
}

func (r *azureRowReader) Next() bool {
	if r.read {
		return false
	}

	r.read = true

	return true
}

func (*azureRowReader) Scan(_ any) error {
	// Implementation would depend on the specific data format
	// For now, this is a placeholder
	return nil
}

// Helper functions.
func getShareName(name string) string {
	parts := strings.Split(name, string(filepath.Separator))
	if len(parts) > 0 {
		return parts[0]
	}

	return ""
}

func getLocation(shareName string) string {
	return fmt.Sprintf("azure://%s", shareName)
}
