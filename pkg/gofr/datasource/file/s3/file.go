package s3

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	file_interface "gofr.dev/pkg/gofr/datasource/file"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
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
	defer f.sendOperationStats(&FileLog{Operation: "ReadAll", Location: location}, time.Now())

	if strings.HasSuffix(f.Name(), ".json") {
		return f.createJSONReader(location)
	}

	return f.createTextCSVReader(location)
}

// createJSONReader creates a JSON reader for JSON files.
func (f *file) createJSONReader(location string) (file_interface.RowReader, error) {
	status := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "JSON Reader", Location: location, Status: &status}, time.Now())

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

	defer f.sendOperationStats(&FileLog{Operation: "Text/CSV Reader", Location: location, Status: &status}, time.Now())

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

func (f *file) Name() string {
	return path.Base(f.name)
}

func (f *file) Mode() os.FileMode {
	return os.ModePerm
}

func (f *file) Size() int64 {
	return f.size
}

func (f *file) ModTime() time.Time {
	return f.lastModified
}

func (f *file) IsDir() bool {
	if f.name[len(f.name)-1] == '/' {
		return true
	}

	return false
}

func (f *file) Close() error {
	if f.body != nil {
		return f.body.Close()
	}

	return nil
}

func (f *file) Read(p []byte) (n int, err error) {
	bucketName := strings.Split(f.name, string(filepath.Separator))[0]
	var fileName string
	index := strings.Index(f.name, string(filepath.Separator))
	if index != -1 {
		fileName = f.name[index+1:]
	}

	res, err := f.conn.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileName),
	})

	if err != nil {
		f.logger.Errorf("Failed to retrieve %q: %v", fileName, err)
		return 0, err
	}

	f.body = res.Body
	if f.body == nil {
		return 0, errors.New("S3 body is nil")
	}

	b := make([]byte, len(p)+int(f.offset))
	n, err = f.body.Read(b)
	if err != nil && err != io.EOF {
		return n, err
	}

	b = b[f.offset:]
	copy(p, b)

	f.offset += int64(len(b))

	return len(p), err
}

func (f *file) ReadAt(p []byte, offset int64) (n int, err error) {
	bucketName := strings.Split(f.name, string(filepath.Separator))[0]
	var fileName string
	index := strings.Index(f.name, string(filepath.Separator))
	if index != -1 {
		fileName = f.name[index+1:]
	}

	res, err := f.conn.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileName),
	})

	if err != nil {
		f.logger.Errorf("Failed to retrieve %q: %v", fileName, err)
		return 0, err
	}

	f.body = res.Body
	if f.body == nil {
		return 0, errors.New("S3 body is nil")
	}

	if int64(len(p))+offset+1 > f.size {
		return 0, errors.New("reading out of range, fetching from offset. Use Seek to rest offset")
	}

	b := make([]byte, len(p)+int(offset)+1)
	n, err = f.body.Read(b)
	if err != nil && err != io.EOF {
		return n, err
	}

	b = b[offset:]
	copy(p, b)

	return len(p), nil
}

func (f *file) Write(p []byte) (n int, err error) {
	bucketName := strings.Split(f.name, string(filepath.Separator))[0]
	var fileName string
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
			f.logger.Errorf("Failed to retrieve %q: %v", fileName, err)
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
		return 0, err
	}

	f.offset += int64(len(p))
	f.size = int64(len(buffer))

	return len(p), nil
}

func (f *file) WriteAt(p []byte, offset int64) (n int, err error) {
	bucketName := strings.Split(f.name, string(filepath.Separator))[0]
	var fileName string
	index := strings.Index(f.name, string(filepath.Separator))
	if index != -1 {
		fileName = f.name[index+1:]
	}

	res, err := f.conn.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileName),
	})

	if err != nil {
		f.logger.Errorf("Failed to retrieve %q: %v", fileName, err)
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
		return 0, err
	}

	f.size = int64(len(buffer))

	return len(p), nil
}

func (f *file) check(whence int, offset, length int64) (int64, error) {
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
		return 0, ErrOutOfRange
	}

	f.offset = offset

	return f.offset, nil
}

// Seek sets the offset for the next Read/ Write operations.
func (f *file) Seek(offset int64, whence int) (int64, error) {
	var msg string

	status := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "Seek", Location: f.name, Status: &status, Message: &msg}, time.Now())

	n := f.Size()
	res, err := f.check(whence, offset, n)
	if err != nil {
		f.logger.Errorf("Seek failed : Error : %v", err)
		return 0, err
	}

	status = "SUCCESS"
	msg = fmt.Sprintf("Offset set to %v for file at path %q", res, f.name)

	return res, nil
}
func (f *file) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	fl.Duration = duration

	f.logger.Debug(fl)
}
