package s3

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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

	file_interface "gofr.dev/pkg/gofr/datasource/file"
)

type file struct {
	conn         *s3.Client
	name         string
	offset       int64
	logger       Logger
	metrics      Metrics
	size         int64
	contentType  string
	body         io.ReadCloser
	lastModified time.Time
}

var (
	// errNotPointer is returned when Read method is called with a non-pointer argument.
	errNotPointer = errors.New("input should be a pointer to a string")
	ErrOutOfRange = errors.New("out of range")
)

// textReader implements RowReader for reading text files.
type textReader struct {
	scanner *bufio.Scanner
	logger  Logger
}

// jsonReader implements RowReader for reading JSON files.
type jsonReader struct {
	decoder *json.Decoder
	token   json.Token
}

// ReadAll reads either JSON or text files based on file extension and returns a corresponding RowReader.
func (f *file) ReadAll() (file_interface.RowReader, error) {
	bucketName := strings.Split(f.name, string(filepath.Separator))[0]

	var fileName string

	index := strings.Index(f.name, string(filepath.Separator))
	if index != -1 {
		fileName = f.name[index+1:]
	}

	location := path.Join(bucketName, fileName)

	defer f.sendOperationStats(&FileLog{Operation: "READALL", Location: location}, time.Now())

	if strings.HasSuffix(f.Name(), ".json") {
		return f.createJSONReader(location)
	}

	return f.createTextCSVReader(location)
}

// createJSONReader creates a JSON reader for JSON files.
func (f *file) createJSONReader(location string) (file_interface.RowReader, error) {
	status := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "JSON READER", Location: location, Status: &status}, time.Now())

	buffer, err := io.ReadAll(f.body)
	if err != nil {
		f.logger.Errorf("ReadAll Failed : Unable to read json file: %v", err)
		return nil, err
	}

	reader := bytes.NewReader(buffer)

	decoder := json.NewDecoder(reader)

	// Peek the first JSON token to determine the type
	// Note: This results in offset to move ahead, making it necessary to
	// decode again if we are decoding a json object instead of array
	token, err := decoder.Token()
	if err != nil {
		f.logger.Errorf("Error decoding token : %v", err)
		return nil, err
	}

	if d, ok := token.(json.Delim); ok && d == '[' {
		status = "SUCCESS"
		return &jsonReader{decoder: decoder, token: token}, err
	}

	// Reading JSON object
	decoder = json.NewDecoder(reader)
	status = "SUCCESS"

	return &jsonReader{decoder: decoder}, nil
}

// createTextCSVReader creates a text reader for reading text files.
func (f *file) createTextCSVReader(location string) (file_interface.RowReader, error) {
	status := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "TEXT/CSV READER", Location: location, Status: &status}, time.Now())

	buffer, err := io.ReadAll(f.body)
	if err != nil {
		f.logger.Errorf("ReadAll failed : Unable to read text file: %v", err)
		return nil, err
	}

	reader := bytes.NewReader(buffer)
	status = "SUCCESS"

	return &textReader{
		scanner: bufio.NewScanner(reader),
		logger:  f.logger,
	}, err
}

// Next checks if there is another JSON object available.
func (j *jsonReader) Next() bool {
	return j.decoder.More()
}

// Scan decodes the next JSON object into the provided structure.
func (j *jsonReader) Scan(i interface{}) error {
	return j.decoder.Decode(&i)
}

// Next checks if there is another line available in the text file.
func (f *textReader) Next() bool {
	return f.scanner.Scan()
}

// Scan scans the next line from the text file into the provided pointer to string.
func (f *textReader) Scan(i interface{}) error {
	if val, ok := i.(*string); ok {
		*val = f.scanner.Text()
		return nil
	}

	return errNotPointer
}

// Name returns the base name of the file.
//
// For a file, this method returns the name of the file without any directory components.
// For directories, it returns the name of the directory.
func (f *file) Name() string {
	bucketName := strings.Split(f.name, string(filepath.Separator))[0]
	location := path.Join(string(filepath.Separator), bucketName)

	defer f.sendOperationStats(&FileLog{Operation: "GET NAME", Location: location}, time.Now())

	return path.Base(f.name)
}

// Mode is not supported for the current implementation of S3 buckets.
// This method is included to adhere to the FileSystem interface in GoFr.
//
// Note: The Mode method does not provide meaningful information for S3 objects
// and should be considered a placeholder in this context.
func (f *file) Mode() os.FileMode {
	bucketName := strings.Split(f.name, string(filepath.Separator))[0]
	location := path.Join(string(filepath.Separator), bucketName)

	defer f.sendOperationStats(&FileLog{Operation: "FILE MODE", Location: location, Message: aws.String("not supported for S3")}, time.Now())

	return os.ModePerm
}

// Size returns the size of the retrieved object.
//
// For files, it returns the size of the file in bytes.
// For directories, it returns the sum of sizes of all files contained within the directory.
//
// Note:
//   - This method should be called on a FileInfo instance obtained from a Stat or ReadDir operation.
func (f *file) Size() int64 {
	bucketName := strings.Split(f.name, string(filepath.Separator))[0]
	location := path.Join(string(filepath.Separator), bucketName)

	defer f.sendOperationStats(&FileLog{Operation: "FILE/DIR SIZE", Location: location}, time.Now())
	return f.size
}

// ModTime returns the last modification time of the file or directory.
//
// For files, it returns the timestamp of the last modification to the file's contents.
// For directories, it returns the timestamp of the most recent change to the directory's contents, including updates
// to files within the directory.
func (f *file) ModTime() time.Time {
	bucketName := strings.Split(f.name, string(filepath.Separator))[0]
	location := path.Join(string(filepath.Separator), bucketName)

	defer f.sendOperationStats(&FileLog{Operation: "LAST MODIFIED", Location: location}, time.Now())
	return f.lastModified
}

// IsDir checks if the FileInfo describes a directory.
//
// This method returns true if the FileInfo object represents a directory; otherwise, it returns false.
// It is specifically used to determine the type of the file system object represented by the FileInfo.
//
// Note:
//   - This method should be called on a FileInfo instance obtained from a Stat or ReadDir operation.
//   - The FileInfo interface is used to describe file system objects, and IsDir is one of its methods to query whether the object is a directory.
func (f *file) IsDir() bool {
	bucketName := strings.Split(f.name, string(filepath.Separator))[0]
	location := path.Join(string(filepath.Separator), bucketName)

	defer f.sendOperationStats(&FileLog{Operation: "IS DIR", Location: location}, time.Now())

	if f.name[len(f.name)-1] == '/' {
		return true
	}

	return false
}

// Close closes the response body returned in Open/Create methods if the response body is not nil.
func (f *file) Close() error {
	bucketName := strings.Split(f.name, string(filepath.Separator))[0]
	location := path.Join(string(filepath.Separator), bucketName)

	defer f.sendOperationStats(&FileLog{Operation: "CLOSE", Location: location}, time.Now())

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
func (f *file) Read(p []byte) (n int, err error) {
	bucketName := strings.Split(f.name, string(filepath.Separator))[0]
	location := path.Join(string(filepath.Separator), bucketName)

	var fileName, msg string

	index := strings.Index(f.name, string(filepath.Separator))
	if index != -1 {
		fileName = f.name[index+1:]
	}

	st := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "READ", Location: location, Status: &st, Message: &msg}, time.Now())

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
		return 0, errors.New("S3 body is nil")
	}

	b := make([]byte, len(p)+int(f.offset))

	n, err = f.body.Read(b)
	if err != nil && err != io.EOF {
		f.logger.Errorf("Error reading file %q: %v", fileName, err)
		return n, err
	}

	b = b[f.offset:]
	copy(p, b)

	f.offset += int64(len(b))

	st = "SUCCESS"
	msg = fmt.Sprintf("Read %v bytes from file at path %q in bucket %q", n, fileName, bucketName)

	f.logger.Logf("%v bytes read successfully)", len(p))

	return len(p), err
}

// ReadAt reads data from the file at a specified offset into the provided byte slice.
//
// This method reads up to len(p) bytes from the file, starting at the given offset. It does not alter the current file offset
// used for other read or write operations. The number of bytes read is returned, along with any error encountered.
func (f *file) ReadAt(p []byte, offset int64) (n int, err error) {
	bucketName := strings.Split(f.name, string(filepath.Separator))[0]
	location := path.Join(string(filepath.Separator), bucketName)

	var fileName, msg string
	st := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "READAT", Location: location, Status: &st, Message: &msg}, time.Now())

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
	if f.body == nil {
		msg = fmt.Sprintf("File %q is nil", fileName)
		return 0, io.EOF
	}

	if int64(len(p))+offset+1 > f.size {
		msg = fmt.Sprintf("Offset %v out of range", f.offset)
		return 0, errors.New("reading out of range, fetching from offset. Use Seek to rest offset")
	}

	b := make([]byte, len(p)+int(offset)+1)

	n, err = f.body.Read(b)
	if err != nil && err != io.EOF {
		return n, err
	}

	b = b[offset:]
	copy(p, b)

	st = "SUCCESS"
	msg = fmt.Sprintf("Read %v bytes at an offset of %v from file at path %q in bucket %q", n, offset, fileName, bucketName)

	f.logger.Logf("%v bytes read successfully.")
	return len(p), nil
}

// Write writes data to the file at the current offset and updates the file offset.
//
// This method writes up to len(p) bytes from the provided byte slice to the file, starting at the current offset.
// It updates the file offset after the write operation to reflect the new position for subsequent read or write operations.
func (f *file) Write(p []byte) (n int, err error) {
	bucketName := strings.Split(f.name, string(filepath.Separator))[0]
	location := path.Join(string(filepath.Separator), bucketName)

	var fileName, msg string
	st := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "WRITE", Location: location, Status: &st, Message: &msg}, time.Now())

	index := strings.Index(f.name, string(filepath.Separator))
	if index != -1 {
		fileName = f.name[index+1:]
	}

	buffer := p

	if f.offset != 0 {
		res, err := f.conn.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(fileName),
		})

		if err != nil {
			msg = fmt.Sprintf("Failed to retrieve file %q: %v", fileName, err)
			return 0, err
		}

		f.body = res.Body

		buffer, _ = io.ReadAll(f.body)

		var buffer1, buffer2 []byte
		if f.offset < f.size {
			buffer1 = buffer[:f.offset]
		}
		if f.offset+int64(len(p)-1) < f.size {
			buffer2 = buffer[f.offset+int64(len(p))-1:]
		}
		buffer = append(buffer1, p...)
		buffer = append(buffer, buffer2...)
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
		msg = fmt.Sprintf("Fsiled to put file %q: %v", fileName, err)
		return 0, err
	}

	f.offset += int64(len(p))
	f.size = int64(len(buffer))

	st = "SUCCESS"
	msg = fmt.Sprintf("Wrote %v bytes to file at path %q in bucket %q", n, fileName, bucketName)

	f.logger.Logf("%v bytes written successfully", len(p))
	return len(p), nil
}

// WriteAt writes data to the file at a specified offset without altering the current file offset.
//
// This method writes up to len(p) bytes from the provided byte slice to the file, starting at the given offset.
// It does not modify the file's current offset used for other read or write operations.
// The number of bytes written and any error encountered during the operation are returned.
func (f *file) WriteAt(p []byte, offset int64) (n int, err error) {
	bucketName := strings.Split(f.name, string(filepath.Separator))[0]
	location := path.Join(string(filepath.Separator), bucketName)

	var fileName, msg string
	st := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "WRITEAT", Location: location, Status: &st, Message: &msg}, time.Now())

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

	buffer, err := io.ReadAll(f.body)
	buffer1 := buffer[:offset-1]
	buffer2 := buffer[offset+int64(len(p))-1:]
	buffer = append(buffer1, p...)
	buffer = append(buffer, buffer2...)

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

	st = "SUCCESS"
	msg = fmt.Sprintf("Wrote %v bytes at an offset of %v to file at path %q in bucket %q", n, offset, fileName, bucketName)

	return len(p), nil
}

// check validates the provided arguments for the Seek method and updates the file offset.
//
// This method performs validation on the arguments provided to the Seek operation. If the arguments are valid, it sets
// the file offset to the specified position. If there are any validation errors, it returns an appropriate error.
func (f *file) check(whence int, offset, length int64, msg *string) (int64, error) {
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
func (f *file) Seek(offset int64, whence int) (int64, error) {
	var msg string
	status := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "SEEK", Location: f.name, Status: &status, Message: &msg}, time.Now())

	n := f.Size()
	res, err := f.check(whence, offset, n, &msg)
	if err != nil {
		f.logger.Errorf("Seek failed. Error : %v", err)
		return 0, err
	}

	status = "SUCCESS"
	msg = fmt.Sprintf("Offset set to %v for file with path %q", res, f.name)

	f.logger.Logf("Set file offset at %v", f.offset)
	return res, nil
}

// sendOperationStats logs the FileLog of any file operations performed in S3.
func (f *file) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	fl.Duration = duration

	f.logger.Debug(fl)
}
