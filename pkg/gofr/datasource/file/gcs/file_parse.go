package gcs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"

	"gofr.dev/pkg/gofr/datasource/file"
)

var (
	errStringNotPointer = errors.New("input should be a pointer to a string")
)

const (
	appFTPStats   = "app_ftp_stats"
	statusErr     = "ERROR"
	statusSuccess = "SUCCESS"
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

func (f *File) ReadAll() (file.RowReader, error) {
	bucketName := getBucketName(f.name)
	location := path.Join(bucketName, f.name)

	defer f.sendOperationStats(&FileLog{
		Operation: "READALL",
		Location:  location,
	}, time.Now())

	if strings.HasSuffix(f.Name(), ".json") {
		return f.createJSONReader(location)
	}

	return f.createTextCSVReader(location)
}

// createJSONReader creates a JSON reader for JSON files.
func (f *File) createJSONReader(location string) (file.RowReader, error) {
	status := statusErr

	defer f.sendOperationStats(&FileLog{Operation: "JSON READER", Location: location, Status: &status}, time.Now())

	buffer, err := io.ReadAll(f.body)
	if err != nil {
		f.logger.Errorf("ReadAll Failed: Unable to read json file: %v", err)
		return nil, err
	}

	reader := bytes.NewReader(buffer)

	decoder := json.NewDecoder(reader)

	token, err := decoder.Token()
	if err != nil {
		f.logger.Errorf("Error decoding token: %v", err)
		return nil, err
	}

	if d, ok := token.(json.Delim); ok && d == '[' {
		status = statusSuccess
		return &jsonReader{decoder: decoder, token: token}, err
	}

	// Reading JSON object
	decoder = json.NewDecoder(reader)
	status = statusSuccess

	return &jsonReader{decoder: decoder}, nil
}

// createTextCSVReader creates a text reader for reading text files.
func (f *File) createTextCSVReader(location string) (file.RowReader, error) {
	status := statusErr

	defer f.sendOperationStats(&FileLog{Operation: "TEXT/CSV READER", Location: location, Status: &status}, time.Now())

	buffer, err := io.ReadAll(f.body)
	if err != nil {
		f.logger.Errorf("ReadAll failed: Unable to read text file: %v", err)
		return nil, err
	}

	reader := bytes.NewReader(buffer)
	status = statusSuccess

	return &textReader{
		scanner: bufio.NewScanner(reader),
		logger:  f.logger,
	}, err
}

func (j *jsonReader) Next() bool {
	return j.decoder.More()
}

// Scan decodes the next JSON object into the provided structure.
func (j *jsonReader) Scan(i any) error {
	return j.decoder.Decode(&i)
}

// Next checks if there is another line available in the text file.
func (f *textReader) Next() bool {
	return f.scanner.Scan()
}

// Scan scans the next line from the text file into the provided pointer to strinf.
func (f *textReader) Scan(i any) error {
	if val, ok := i.(*string); ok {
		*val = f.scanner.Text()
		return nil
	}

	return errStringNotPointer
}

func (f *File) Name() string {
	bucketName := getBucketName(f.name)

	f.sendOperationStats(&FileLog{
		Operation: "GET NAME",
		Location:  getLocation(bucketName),
	}, time.Now())

	return f.name
}

func (f *File) Size() int64 {
	bucketName := getBucketName(f.name)

	f.sendOperationStats(&FileLog{
		Operation: "FILE/DIR SIZE",
		Location:  getLocation(bucketName),
	}, time.Now())

	return f.size
}

func (f *File) ModTime() time.Time {
	bucketName := getBucketName(f.name)

	f.sendOperationStats(&FileLog{
		Operation: "LAST MODIFIED",
		Location:  getLocation(bucketName),
	}, time.Now())

	return f.lastModified
}

func (f *File) Mode() fs.FileMode {
	bucketName := getBucketName(f.name)

	f.sendOperationStats(&FileLog{
		Operation: "MODE",
		Location:  getLocation(bucketName),
	}, time.Now())

	if f.isDir {
		return fs.ModeDir
	}

	return 0
}

func (f *File) IsDir() bool {
	bucketName := getBucketName(f.name)

	f.sendOperationStats(&FileLog{
		Operation: "IS DIR",
		Location:  getLocation(bucketName),
	}, time.Now())

	return f.isDir || f.contentType == "application/x-directory"
}

func (f *File) Sys() any {
	bucketName := getBucketName(f.name)

	f.sendOperationStats(&FileLog{
		Operation: "SYS",
		Location:  getLocation(bucketName),
	}, time.Now())

	return nil
}
