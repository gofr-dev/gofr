package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3File represents a file in an S3 bucket.
//
//nolint:revive // s3.S3File is repetitive. A better name could have been chosen, but it's too late as it's already exported.
type S3File struct {
	conn         s3Client
	name         string
	offset       int64
	logger       Logger
	metrics      Metrics
	size         int64
	contentType  string
	body         io.ReadCloser
	lastModified time.Time
}

func (*S3File) Sys() any {
	return nil
}

var (
	ErrNilResponse = errors.New("response retrieved is nil ")
)

// Close closes the response body returned in Open/Create methods if the response body is not nil.
func (f *S3File) Close() error {
	bucketName := getBucketName(f.name)

	defer f.sendOperationStats(&FileLog{
		Operation: "CLOSE",
		Location:  getLocation(bucketName)}, time.Now())

	if f.body != nil {
		return f.body.Close()
	}

	return nil
}

// Read reads data into the provided byte slice.
//
// It attempts to fill the entire slice with data from the file. If the number of bytes read is less than the length of the slice,
// it returns an io.EOF error to indicate that the end of the file has been reached before the slice was completely filled.
//
// Additionally, this method updates the file offset to reflect the next position that will be used for subsequent read or write operations.
func (f *S3File) Read(p []byte) (n int, err error) {
	var fileName, msg string

	bucketName := getBucketName(f.name)

	// get path relative to current bucketName
	index := strings.Index(f.name, string(filepath.Separator))
	if index != -1 {
		fileName = f.name[index+1:]
	}

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "READ",
		Location:  getLocation(bucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	res, err := f.conn.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileName),
	})
	if err != nil {
		msg = fmt.Sprintf("Failed to retrieve %q: %v", fileName, err)
		return 0, err
	}

	f.body = res.Body
	if f.body == nil {
		msg = fmt.Sprintf("File %q is nil", fileName)
		return 0, fmt.Errorf("%w: S3 file is empty", ErrNilResponse)
	}

	buffer := make([]byte, len(p)+int(f.offset))

	n, err = f.body.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		f.logger.Errorf("Error reading file %q: %v", fileName, err)
		return n, err
	}

	buffer = buffer[f.offset:]
	copy(p, buffer)

	f.offset += int64(len(buffer))

	st = statusSuccess
	msg = fmt.Sprintf("Read %v bytes from file at path %q in bucket %q", n, fileName, bucketName)

	f.logger.Logf("%v bytes read successfully)", len(p))

	return len(p), err
}

// ReadAt reads data from the file at a specified offset into the provided byte slice.
//
// This method reads up to len(p) bytes from the file, starting at the given offset. It does not alter the current file offset
// used for other read or write operations. The number of bytes read is returned, along with any error encountered.
func (f *S3File) ReadAt(p []byte, offset int64) (n int, err error) {
	bucketName := getBucketName(f.name)

	var fileName, msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "READAT",
		Location:  getLocation(bucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	// get path relative to current bucketName
	index := strings.Index(f.name, string(filepath.Separator))
	if index != -1 {
		fileName = f.name[index+1:]
	}

	var res *s3.GetObjectOutput

	res, err = f.conn.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileName),
	})
	if err != nil {
		msg = fmt.Sprintf("Failed to retrieve file %q: %v", fileName, err)
		return 0, err
	}

	f.body = res.Body
	if f.body == nil {
		msg = fmt.Sprintf("File %q is nil", fileName)
		return 0, io.EOF
	}

	if int64(len(p))+offset+1 > f.size {
		msg = fmt.Sprintf("Offset %v out of range", f.offset)
		return 0, fmt.Errorf("%w: reading out of range, fetching from the offset. Use Seek to reset offset", ErrOutOfRange)
	}

	buffer := make([]byte, len(p)+int(offset)+1)

	n, err = f.body.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, err
	}

	buffer = buffer[offset:]
	copy(p, buffer)

	st = statusSuccess
	msg = fmt.Sprintf("Read %v bytes at an offset of %v from file at path %q in bucket %q", n, offset, fileName, bucketName)

	f.logger.Logf("%v bytes read successfully.")

	return len(p), nil
}

// Write writes data to the file at the current offset and updates the file offset.
//
// This method writes up to len(p) bytes from the provided byte slice to the file, starting at the current offset.
// It updates the file offset after the write operation to reflect the new position for subsequent read or write operations.
func (f *S3File) Write(p []byte) (n int, err error) {
	bucketName := getBucketName(f.name)

	var fileName, msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{
		Operation: "WRITE",
		Location:  getLocation(bucketName),
		Status:    &st,
		Message:   &msg,
	}, time.Now())

	// extracting file name
	index := strings.Index(f.name, string(filepath.Separator))
	if index != -1 {
		fileName = f.name[index+1:]
	}

	buffer := p

	var res *s3.GetObjectOutput

	// if f.offset is not 0, we need to fetch the contents of the file till the offset and then write into the file
	if f.offset != 0 {
		res, err = f.conn.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(fileName),
		})
		if err != nil {
			msg = fmt.Sprintf("Failed to retrieve file %q: %v", fileName, err)
			return 0, err
		}

		f.body = res.Body

		buffer, err = io.ReadAll(f.body)
		if err != nil && !errors.Is(err, io.EOF) {
			msg = fmt.Sprintf("Failed to read file %q to perform write at offset of %v: %v", fileName, f.offset, err)
			return 0, err
		}

		var contentBeforeOffset, contentAfterBufferBytes []byte

		contentBeforeOffset = buffer[:f.offset]

		if f.offset+int64(len(p)) < f.size {
			contentAfterBufferBytes = buffer[f.offset+int64(len(p)):]
		}

		contentBeforeOffset = append(contentBeforeOffset, p...)
		buffer = contentBeforeOffset
		buffer = append(buffer, contentAfterBufferBytes...)
	}

	_, err = f.conn.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(fileName),
		Body:        bytes.NewReader(buffer),
		ContentType: aws.String(mime.TypeByExtension(path.Ext(f.name))),
		// this specifies the file must be downloaded before being opened
		ContentDisposition: aws.String("attachment"),
	})
	if err != nil {
		msg = fmt.Sprintf("Failed to put file %q: %v", fileName, err)
		return 0, err
	}

	f.offset += int64(len(p))
	f.size = int64(len(buffer))

	st = statusSuccess
	msg = fmt.Sprintf("Wrote %v bytes to file at path %q in bucket %q", n, fileName, bucketName)

	f.logger.Logf("%v bytes written successfully", len(p))

	return len(p), nil
}

// WriteAt writes data to the file at a specified offset without altering the current file offset.
//
// This method writes up to len(p) bytes from the provided byte slice to the file, starting at the given offset.
// It does not modify the file's current offset used for other read or write operations.
// The number of bytes written and any error encountered during the operation are returned.
func (f *S3File) WriteAt(p []byte, offset int64) (n int, err error) {
	bucketName := getBucketName(f.name)

	var fileName, msg string

	st := statusErr

	defer f.sendOperationStats(&FileLog{Operation: "WRITEAT", Location: getLocation(bucketName), Status: &st, Message: &msg}, time.Now())

	index := strings.Index(f.name, string(filepath.Separator))
	if index != -1 {
		fileName = f.name[index+1:]
	}

	res, err := f.conn.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileName),
	})
	if err != nil {
		msg = fmt.Sprintf("Failed to retrieve file %q: %v", fileName, err)
		return 0, err
	}

	f.body = res.Body

	var contentAfterBufferBytes []byte

	buffer, err := io.ReadAll(f.body)

	contentBeforeOffset := buffer[:offset]

	if offset+int64(len(p)) < f.size {
		contentAfterBufferBytes = buffer[offset+int64(len(p)):]
	}

	contentBeforeOffset = append(contentBeforeOffset, p...)
	buffer = contentBeforeOffset
	buffer = append(buffer, contentAfterBufferBytes...)

	_, err = f.conn.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(fileName),
		Body:        bytes.NewReader(buffer),
		ContentType: aws.String(mime.TypeByExtension(path.Ext(f.name))),
		// this specifies the file must be downloaded before being opened
		ContentDisposition: aws.String("attachment"),
	})
	if err != nil {
		msg = fmt.Sprintf("Failed to put file %q: %v", fileName, err)
		return 0, err
	}

	f.size = int64(len(buffer))

	st = statusSuccess
	msg = fmt.Sprintf("Wrote %v bytes at an offset of %v to file at path %q in bucket %q", n, offset, fileName, bucketName)

	return len(p), nil
}

// check validates the provided arguments for the Seek method and updates the file offset.
//
// This method performs validation on the arguments provided to the Seek operation. If the arguments are valid, it sets
// the file offset to the specified position. If there are any validation errors, it returns an appropriate error.
func (f *S3File) check(whence int, offset, length int64, msg *string) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekEnd:
		offset += length
	case io.SeekCurrent:
		offset += f.offset
	default:
		return 0, os.ErrInvalid
	}

	if offset < 0 || offset > length {
		*msg = fmt.Sprintf("Offset %v out of bounds %v", offset, length)
		return 0, ErrOutOfRange
	}

	f.offset = offset

	return f.offset, nil
}

// Seek sets the file offset to a specified position and returns the new offset.
//
// This method changes the file's current offset to the given position based on the reference point specified by the `whence` parameter.
// It uses the provided offset and whence values to determine the new file position. The method returns the new offset
// and any error encountered during the operation.
//
// Parameters:
//
//	offset int64: The desired position to seek to in the file.
//	whence int: The reference point for the new offset. It can be one of the following:
//	  - `io.SeekStart` (0): Offset is relative to the start of the file.
//	  - `io.SeekCurrent` (1): Offset is relative to the current position in the file.
//	  - `io.SeekEnd` (2): Offset is relative to the end of the file.
func (f *S3File) Seek(offset int64, whence int) (int64, error) {
	var msg string

	status := statusErr

	defer f.sendOperationStats(&FileLog{Operation: "SEEK", Location: f.name, Status: &status, Message: &msg}, time.Now())

	n := f.Size()

	res, err := f.check(whence, offset, n, &msg)
	if err != nil {
		f.logger.Errorf("Seek failed. Error: %v", err)
		return 0, err
	}

	status = statusSuccess
	msg = fmt.Sprintf("Offset set to %v for file with path %q", res, f.name)

	f.logger.Logf("Set file offset at %v", f.offset)

	return res, nil
}

// sendOperationStats logs the FileLog of any file operations performed in S3.
func (f *S3File) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Microseconds()

	fl.Duration = duration

	f.logger.Debug(fl)
}
