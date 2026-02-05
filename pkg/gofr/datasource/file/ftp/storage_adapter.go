package ftp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"gofr.dev/pkg/gofr/datasource/file"
)

var (
	// Storage adapter errors.
	errFTPConfigNil            = errors.New("FTP config is nil")
	errFTPClientNotInitialized = errors.New("FTP client is not initialized")
	errEmptyObjectName         = errors.New("object name is empty")
	errInvalidOffset           = errors.New("invalid offset: must be >= 0")
	errEmptySourceOrDest       = errors.New("source and destination names cannot be empty")
	errSameSourceAndDest       = errors.New("source and destination are the same")
	errFailedToCreateReader    = errors.New("failed to create reader")
	errFailedToCreateWriter    = errors.New("failed to create writer")
	errObjectNotFound          = errors.New("object not found")
	errFailedToGetObjectAttrs  = errors.New("failed to get object attrs")
	errFailedToDeleteObject    = errors.New("failed to delete object")
	errFailedToListObjects     = errors.New("failed to list objects")
	errFailedToListDirectory   = errors.New("failed to list directory")
	errWriterAlreadyClosed     = errors.New("writer already closed")
	errFTPConfigInvalid        = errors.New("invalid FTP configuration: host and port are required")
)

// Config represents the FTP configuration.
type Config struct {
	Host        string        // FTP server hostname
	User        string        // FTP username
	Password    string        // FTP password
	Port        int           // FTP port
	RemoteDir   string        // Remote directory path. Base Path for all FTP Operations.
	DialTimeout time.Duration // FTP connection timeout
}

// storageAdapter adapts FTP client to implement file.StorageProvider.
type storageAdapter struct {
	cfg  *Config
	conn *ftp.ServerConn
}

// Connect initializes the FTP client and logs in to the server.
func (s *storageAdapter) Connect(_ context.Context) error {
	// fast-path: already connected
	if s.conn != nil {
		return nil
	}

	if s.cfg == nil {
		return errFTPConfigNil
	}

	if s.cfg.Host == "" || s.cfg.Port <= 0 {
		return errFTPConfigInvalid
	}

	// Set default timeout if not specified
	dialTimeout := s.cfg.DialTimeout
	if dialTimeout == 0 {
		dialTimeout = 5 * time.Second
	}

	ftpServer := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	conn, err := ftp.Dial(ftpServer, ftp.DialWithTimeout(dialTimeout))
	if err != nil {
		return fmt.Errorf("failed to dial FTP server %q: %w", ftpServer, err)
	}

	if err := conn.Login(s.cfg.User, s.cfg.Password); err != nil {
		_ = conn.Quit()
		return fmt.Errorf("FTP login failed for user %q: %w", s.cfg.User, err)
	}

	s.conn = conn

	return nil
}

// NewReader creates a reader for the given object.
func (s *storageAdapter) NewReader(_ context.Context, name string) (io.ReadCloser, error) {
	if name == "" {
		return nil, errEmptyObjectName
	}

	if s.conn == nil {
		return nil, errFTPClientNotInitialized
	}

	objectPath := s.buildPath(name)

	reader, err := s.conn.Retr(objectPath)
	if err != nil {
		if isFTPNotFoundError(err) {
			return nil, fmt.Errorf("%w %q: %w", errObjectNotFound, name, err)
		}

		return nil, fmt.Errorf("%w for %q: %w", errFailedToCreateReader, name, err)
	}

	return reader, nil
}

// NewRangeReader creates a range reader for the given object.
func (s *storageAdapter) NewRangeReader(_ context.Context, name string, offset, length int64) (io.ReadCloser, error) {
	if name == "" {
		return nil, errEmptyObjectName
	}

	if offset < 0 {
		return nil, fmt.Errorf("%w (got: %d)", errInvalidOffset, offset)
	}

	if s.conn == nil {
		return nil, errFTPClientNotInitialized
	}

	objectPath := s.buildPath(name)

	reader, err := s.conn.RetrFrom(objectPath, uint64(offset))
	if err != nil {
		if isFTPNotFoundError(err) {
			return nil, fmt.Errorf("%w %q: %w", errObjectNotFound, name, err)
		}

		return nil, fmt.Errorf("%w for %q at offset %d: %w", errFailedToCreateReader, name, offset, err)
	}

	// FTP doesn't support length limit in RetrFrom, so wrap in LimitReader
	if length > 0 {
		return &limitedReadCloser{
			Reader: io.LimitReader(reader, length),
			Closer: reader,
		}, nil
	}

	return reader, nil
}

// limitedReadCloser combines io.LimitReader with Close capability.
type limitedReadCloser struct {
	io.Reader
	io.Closer
}

// NewWriter creates a writer for the given object.
func (s *storageAdapter) NewWriter(_ context.Context, name string) io.WriteCloser {
	if name == "" {
		return &failWriter{err: errEmptyObjectName}
	}

	if s.conn == nil {
		return &failWriter{err: errFTPClientNotInitialized}
	}

	objectPath := s.buildPath(name)

	return &ftpWriter{
		conn:       s.conn,
		objectPath: objectPath,
		buffer:     &bytes.Buffer{},
	}
}

// ftpWriter buffers writes and uploads on Close.
type ftpWriter struct {
	conn       *ftp.ServerConn
	objectPath string
	buffer     *bytes.Buffer
	closed     bool
}

func (fw *ftpWriter) Write(p []byte) (int, error) {
	if fw.closed {
		return 0, errWriterAlreadyClosed
	}

	return fw.buffer.Write(p)
}

func (fw *ftpWriter) Close() error {
	if fw.closed {
		return nil
	}

	fw.closed = true

	if err := fw.conn.Stor(fw.objectPath, fw.buffer); err != nil {
		return fmt.Errorf("%w for %q: %w", errFailedToCreateWriter, fw.objectPath, err)
	}

	return nil
}

// failWriter is a helper for NewWriter validation errors.
type failWriter struct {
	err error
}

func (fw *failWriter) Write([]byte) (int, error) {
	return 0, fw.err
}

func (fw *failWriter) Close() error {
	return fw.err
}

// DeleteObject deletes the object with the given name.
func (s *storageAdapter) DeleteObject(_ context.Context, name string) error {
	if name == "" {
		return errEmptyObjectName
	}

	if s.conn == nil {
		return errFTPClientNotInitialized
	}

	objectPath := s.buildPath(name)

	if err := s.conn.Delete(objectPath); err != nil {
		if isFTPNotFoundError(err) {
			return fmt.Errorf("%w %q: %w", errObjectNotFound, name, err)
		}

		return fmt.Errorf("%w %q: %w", errFailedToDeleteObject, name, err)
	}

	return nil
}

// CopyObject copies an object from src to dst.
func (s *storageAdapter) CopyObject(_ context.Context, source, dest string) error {
	if source == "" || dest == "" {
		return errEmptySourceOrDest
	}

	if source == dest {
		return errSameSourceAndDest
	}

	if s.conn == nil {
		return errFTPClientNotInitialized
	}

	// Read source file
	sourcePath := s.buildPath(source)

	resp, err := s.conn.Retr(sourcePath)
	if err != nil {
		if isFTPNotFoundError(err) {
			return fmt.Errorf("%w: %q", errObjectNotFound, source)
		}

		return fmt.Errorf("failed to read source object %q: %w", source, err)
	}

	// Read all data from source into memory
	data, err := io.ReadAll(resp)

	closeErr := resp.Close()

	if err != nil {
		return fmt.Errorf("failed to read source object data %q: %w", source, err)
	}

	if closeErr != nil {
		return fmt.Errorf("failed to close source reader for %q: %w", source, closeErr)
	}

	// Write to destination
	destPath := s.buildPath(dest)
	if err := s.conn.Stor(destPath, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("failed to write destination object %q: %w", dest, err)
	}

	return nil
}

// StatObject returns metadata for the given object.
func (s *storageAdapter) StatObject(_ context.Context, name string) (*file.ObjectInfo, error) {
	if name == "" {
		return nil, errEmptyObjectName
	}

	if s.conn == nil {
		return nil, errFTPClientNotInitialized
	}

	objectPath := s.buildPath(name)

	entries, err := s.conn.List(objectPath)
	if err != nil {
		if isFTPNotFoundError(err) {
			return nil, fmt.Errorf("%w %q: %w", errObjectNotFound, name, err)
		}

		return nil, fmt.Errorf("%w for %q: %w", errFailedToGetObjectAttrs, name, err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("%w %q", errObjectNotFound, name)
	}

	entry := entries[0]

	return &file.ObjectInfo{
		Name:         entry.Name,
		Size:         safeUint64ToInt64(entry.Size),
		ContentType:  getContentType(entry),
		LastModified: entry.Time,
		IsDir:        entry.Type == ftp.EntryTypeFolder,
	}, nil
}

// ListObjects lists all objects with the given prefix.
func (s *storageAdapter) ListObjects(_ context.Context, prefix string) ([]string, error) {
	if s.conn == nil {
		return nil, errFTPClientNotInitialized
	}

	dirPath := s.buildPath(prefix)

	// If prefix is empty or is the base directory, list from RemoteDir
	if prefix == "" || prefix == "." {
		dirPath = s.cfg.RemoteDir
	}

	entries, err := s.conn.List(dirPath)
	if err != nil {
		if isFTPNotFoundError(err) {
			return []string{}, nil // Return empty list for non-existent directories
		}

		return nil, fmt.Errorf("%w with prefix %q: %w", errFailedToListObjects, prefix, err)
	}

	var objects []string

	for _, entry := range entries {
		if entry.Type != ftp.EntryTypeFolder {
			// Build relative path from RemoteDir
			relativePath := entry.Name
			if prefix != "" && prefix != "." {
				relativePath = path.Join(prefix, entry.Name)
			}

			objects = append(objects, relativePath)
		}
	}

	return objects, nil
}

// ListDir lists objects and prefixes (directories) under the given prefix.
func (s *storageAdapter) ListDir(_ context.Context, prefix string) ([]file.ObjectInfo, []string, error) {
	if s.conn == nil {
		return nil, nil, errFTPClientNotInitialized
	}

	dirPath := s.resolveDirPath(prefix)

	entries, err := s.conn.List(dirPath)
	if err != nil {
		return s.handleListError(err, prefix)
	}

	return s.processEntries(entries, prefix)
}

// resolveDirPath resolves the directory path for listing.
func (s *storageAdapter) resolveDirPath(prefix string) string {
	if prefix == "" || prefix == "." {
		return s.cfg.RemoteDir
	}

	return s.buildPath(prefix)
}

// handleListError handles errors from List operation.
func (*storageAdapter) handleListError(err error, prefix string) ([]file.ObjectInfo, []string, error) {
	if isFTPNotFoundError(err) {
		return []file.ObjectInfo{}, []string{}, nil
	}

	return nil, nil, fmt.Errorf("%w %q: %w", errFailedToListDirectory, prefix, err)
}

// processEntries processes FTP entries into objects and prefixes.
func (s *storageAdapter) processEntries(entries []*ftp.Entry, prefix string) ([]file.ObjectInfo, []string, error) {
	var (
		objects  []file.ObjectInfo
		prefixes []string
	)

	for _, entry := range entries {
		if entry.Type == ftp.EntryTypeFolder {
			prefixes = append(prefixes, s.buildDirPrefix(entry.Name, prefix))
		} else {
			objects = append(objects, s.buildObjectInfo(entry, prefix))
		}
	}

	return objects, prefixes, nil
}

// buildDirPrefix constructs a directory prefix.
func (*storageAdapter) buildDirPrefix(name, prefix string) string {
	if prefix != "" && prefix != "." {
		return path.Join(prefix, name) + "/"
	}

	return name + "/"
}

// buildObjectInfo constructs file.ObjectInfo from FTP entry.
func (*storageAdapter) buildObjectInfo(entry *ftp.Entry, prefix string) file.ObjectInfo {
	objectName := entry.Name
	if prefix != "" && prefix != "." {
		objectName = path.Join(prefix, entry.Name)
	}

	return file.ObjectInfo{
		Name:         objectName,
		Size:         safeUint64ToInt64(entry.Size),
		ContentType:  getContentType(entry),
		LastModified: entry.Time,
		IsDir:        false,
	}
}

// buildPath constructs the full path by joining RemoteDir with the given name.
func (s *storageAdapter) buildPath(name string) string {
	if s.cfg.RemoteDir == "" || s.cfg.RemoteDir == "/" {
		return name
	}

	return path.Join(s.cfg.RemoteDir, name)
}

// isFTPNotFoundError checks if the error indicates a "not found" condition.
func isFTPNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Common FTP "not found" error patterns
	return strings.Contains(errStr, "550") || // File not found
		strings.Contains(errStr, "551") || // File not available
		strings.Contains(errStr, "No such file") ||
		strings.Contains(errStr, "not found")
}

// getContentType determines content type based on file extension or type.
func getContentType(entry *ftp.Entry) string {
	if entry.Type == ftp.EntryTypeFolder {
		return "application/x-directory"
	}

	ext := strings.ToLower(path.Ext(entry.Name))

	contentTypes := map[string]string{
		".json": "application/json",
		".xml":  "application/xml",
		".txt":  "text/plain; charset=utf-8",
		".csv":  "text/csv",
		".html": "text/html",
		".htm":  "text/html",
		".pdf":  "application/pdf",
		".zip":  "application/zip",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
	}

	if contentType, ok := contentTypes[ext]; ok {
		return contentType
	}

	return "application/octet-stream"
}

func safeUint64ToInt64(u uint64) int64 {
	const maxInt64 = 1<<63 - 1

	if u > maxInt64 {
		return maxInt64
	}

	return int64(u)
}
