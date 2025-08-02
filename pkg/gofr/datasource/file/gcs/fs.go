package gcs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	file "gofr.dev/pkg/gofr/datasource/file"
	"google.golang.org/api/option"
)

const (
	typeFile      = "file"
	typeDirectory = "directory"
)

var (
	errIncorrectFileType = errors.New("incorrect file type")
)

type client struct {
	*storage.Client
}
type FileSystem struct {
	GCSFile GCSFile
	conn    gcsClient
	config  *Config
	logger  Logger
	metrics Metrics
}

// Config represents the s3 configuration.
type Config struct {
	BucketName      string
	CredentialsJSON string
	ProjectID       string
}

// New initializes a new instance of FTP fileSystem with provided configuration.
func New(config *Config) file.FileSystemProvider {
	return &FileSystem{config: config}
}

func (f *FileSystem) Connect() {
	var msg string
	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "CONNECT",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	f.logger.Debugf("connecting to GCS bucket: %s", f.config.BucketName)
	ctx := context.TODO()
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON([]byte(f.config.CredentialsJSON)))

	if err != nil {
		f.logger.Errorf("Failed to connect to GCS: %v", err)
		return
	}
	f.conn = &gcsClientImpl{
		client: client,
		bucket: client.Bucket(f.config.BucketName),
	}
	st = statusSuccess
	msg = "GCS Client connected."
	f.logger.Logf("connected to GCS bucket %s", f.config.BucketName)
}

// func (f *FileSystem) Create(name string) (file.File, error) {

// 	var msg string

// 	st := statusErr

// 	defer f.sendOperationStats(&FileLog{
// 		Operation: "CREATE FILE",
// 		Location:  getLocation(f.config.BucketName),
// 		Status:    &st,
// 		Message:   &msg,
// 	}, time.Now())

// 	ctx := context.TODO()
// 	writer := f.conn.NewWriter(ctx, name)

// 	// write empty data to simulate file
// 	if _, err := writer.Write([]byte{}); err != nil {
// 		fmt.Sprintf(err.Error())
// 		return nil, err
// 	}
// 	err := writer.Close()
// 	if err != nil {
// 		return nil, err
// 	}
// 	st = statusSuccess
// 	msg = "File creation on GCS successful."

// 	f.logger.Logf("File with name %s created.", name)

// 	return f.Open(name) // re-open to return as file.File

// }
// func (f *FileSystem) Create(name string) (file.File, error) {
// 	var msg string
// 	st := statusErr

// 	defer f.sendOperationStats(&FileLog{
// 		Operation: "CREATE FILE",
// 		Location:  getLocation(f.config.BucketName),
// 		Status:    &st,
// 		Message:   &msg,
// 	}, time.Now())

// 	ctx := context.TODO()
// 	writer := f.conn.NewWriter(ctx, name)
// 	if _, err := writer.Write([]byte{}); err != nil {
// 		fmt.Sprintf(err.Error())
// 		return nil, err
// 	}
// 	err := writer.Close()
// 	if err != nil {
// 		return nil, err
// 	}
// 	st = statusSuccess
// 	msg = "Write stream opened successfully"

//		return &GCSFile{
//			writer: writer,
//			name:   name,
//		}, nil
//	}
// func (f *FileSystem) Create(name string) (file.File, error) {
// 	var msg string
// 	st := statusErr

// 	defer f.sendOperationStats(&FileLog{
// 		Operation: "CREATE FILE",
// 		Location:  getLocation(f.config.BucketName),
// 		Status:    &st,
// 		Message:   &msg,
// 	}, time.Now())

// 	ctx := context.TODO()

// 	parentPath := path.Dir(name)

// 	if parentPath != "." {
// 		objects, err := f.conn.ListObjects(ctx, parentPath+"/")
// 		if err != nil {
// 			return nil, err
// 		}
// 		if len(objects) == 0 {
// 			f.logger.Errorf("Parent path %q does not exist", parentPath)
// 			return nil, fmt.Errorf("%w: create parent path before creating a file", ErrOperationNotPermitted)
// 		}
// 	}

// 	// Handle name conflict by checking existence and appending " copy"
// 	originalName := name
// 	index := 1

// 	for {
// 		objects, err := f.conn.ListObjects(ctx, name)
// 		if err != nil {
// 			return nil, err
// 		}
// 		if len(objects) == 0 {
// 			msg = "Write stream opened successfully" //addd a mesage "as already exit ,create file with copy"
// 			break                                    // file doesn't exist, safe to use
// 		}
// 		name = generateCopyName(originalName, index)
// 		index++
// 	}
// 	writer := f.conn.NewWriter(ctx, name)

// 	st = statusSuccess
// 	msg = "Write stream opened successfully"

// 	return &GCSFile{
// 		conn:         f.conn,
// 		writer:       writer,
// 		name:         name,
// 		contentType:  writer.ContentType,
// 		size:         writer.Size,
// 		lastModified: writer.Updated,
// 		logger:       f.logger,
// 		metrics:      f.metrics,
// 	}, nil
// }

// func (f *FileSystem) Create(name string) (file.File, error) {
// 	var msg string
// 	st := statusErr

// 	defer f.sendOperationStats(&FileLog{
// 		Operation: "CREATE FILE",
// 		Location:  getLocation(f.config.BucketName),
// 		Status:    &st,
// 		Message:   &msg,
// 	}, time.Now())

// 	ctx := context.TODO()

// 	parentPath := path.Dir(name)

// 	// 1. First check parent directory (or root)
// 	checkPath := "."
// 	if parentPath != "." {
// 		checkPath = parentPath + "/"
// 	}

// 	_, err := f.conn.ListObjects(ctx, checkPath)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// 2. Handle name conflicts
// 	originalName := name
// 	index := 1
// 	for {
// 		objects, err := f.conn.ListObjects(ctx, name)
// 		if err != nil {
// 			return nil, err
// 		}
// 		if len(objects) == 0 {
// 			break // file doesn't exist, safe to use
// 		}
// 		name = generateCopyName(originalName, index)
// 		index++
// 	}

// 	// 3. Create the file
// 	writer := f.conn.NewWriter(ctx, name)
// 	st = statusSuccess
// 	msg = "Write stream opened successfully"

//		return &GCSFile{
//			conn:         f.conn,
//			writer:       writer,
//			name:         name,
//			contentType:  writer.ContentType,
//			size:         writer.Size,
//			lastModified: writer.Updated,
//			logger:       f.logger,
//			metrics:      f.metrics,
//		}, nil
//	}
func (f *FileSystem) Create(name string) (file.File, error) {
	var (
		msg string
		st  = statusErr
	)

	startTime := time.Now()
	defer f.sendOperationStats(&FileLog{
		Operation: "CREATE FILE",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, startTime)

	ctx := context.Background()

	// 1. Check if parent directory exists
	parentPath := path.Dir(name)
	checkPath := "."
	if parentPath != "." {
		checkPath = parentPath + "/"
	}

	if _, err := f.conn.ListObjects(ctx, checkPath); err != nil {
		msg = "Parent directory does not exist"
		return nil, err
	}

	// 2. Resolve file name conflict
	originalName := name
	for index := 1; ; index++ {
		objs, err := f.conn.ListObjects(ctx, name)
		if err != nil {
			msg = "Error checking existing objects"
			return nil, err
		}
		if len(objs) == 0 {
			break // Safe to use
		}
		name = generateCopyName(originalName, index)
	}

	// 3. Open writer to create file
	writer := f.conn.NewWriter(ctx, name)

	sw, ok := writer.(*storage.Writer)
	if !ok {
		msg = "Failed to assert writer to *storage.Writer"
		return nil, errors.New(msg)
	}

	st = statusSuccess
	msg = "Write stream opened successfully"

	return &GCSFile{
		conn:         f.conn,
		writer:       sw,
		name:         name,
		contentType:  sw.ContentType,
		size:         sw.Size,
		lastModified: sw.Updated,
		logger:       f.logger,
		metrics:      f.metrics,
	}, nil

}

func (f *FileSystem) Remove(name string) error {
	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "REMOVE FILE",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	ctx := context.TODO()
	err := f.conn.DeleteObject(ctx, name)
	if err != nil {
		f.logger.Errorf("Error while deleting file: %v", err)
		return err
	}

	st = statusSuccess
	msg = "File deletion on GCS successful"

	f.logger.Logf("File with path %q deleted", name)

	return nil
}

func (f *FileSystem) Open(name string) (file.File, error) {

	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "OPEN FILE",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	ctx := context.TODO()

	reader, err := f.conn.NewReader(ctx, name)

	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, file.ErrFileNotFound
		}

		f.logger.Errorf("failed to retrieve %q: %v", name, err)

		return nil, err
	}

	attr, err := f.conn.StatObject(ctx, name)
	if err != nil {
		reader.Close()
		return nil, err
	}
	st = statusSuccess
	msg = fmt.Sprintf("File with path %q retrieved successfully", name)

	return &GCSFile{
		conn:         f.conn,
		name:         name,
		body:         reader,
		logger:       f.logger,
		metrics:      f.metrics,
		size:         attr.Size,
		contentType:  attr.ContentType,
		lastModified: attr.Updated,
	}, nil
}

func (f *FileSystem) Rename(oldname, newname string) error {

	var msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "RENAME",
		Location:  getLocation(f.config.BucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	ctx := context.TODO()

	if oldname == newname {
		f.logger.Logf("%q & %q are same", oldname, newname)
		return nil
	}

	if path.Dir(oldname) != path.Dir(newname) {
		f.logger.Errorf("%q & %q are not in same location", oldname, newname)
		return fmt.Errorf("%w: renaming as well as moving file to different location is not allowed", ErrOperationNotPermitted)
	}
	// Copy old object to new
	if err := f.conn.CopyObject(ctx, oldname, newname); err != nil {
		msg = fmt.Sprintf("Error while copying file: %v", err)
		return err
	}

	// Delete old
	err := f.conn.DeleteObject(ctx, oldname)
	if err != nil {
		msg = fmt.Sprintf("failed to remove old file %s", oldname)
		return err
	}

	st = statusSuccess
	msg = "File renamed successfully"

	f.logger.Logf("File with path %q renamed to %q", oldname, newname)

	return nil
}
func (f *FileSystem) OpenFile(name string, _ int, _ os.FileMode) (file.File, error) {
	return f.Open(name)
}

// UseLogger sets the Logger interface for the FTP file system.
func (f *FileSystem) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		f.logger = l
	}
}

// UseMetrics sets the Metrics interface.
func (f *FileSystem) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		f.metrics = m
	}
}
func generateCopyName(original string, count int) string {
	ext := path.Ext(original)
	base := strings.TrimSuffix(original, ext)
	return fmt.Sprintf("%s copy %d%s", base, count, ext)
}
