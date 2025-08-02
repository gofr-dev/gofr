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

	file "gofr.dev/pkg/gofr/datasource/file"
)

var (
	// errNotPointer is returned when Read method is called with a non-pointer argument.
	errStringNotPointer = errors.New("input should be a pointer to a string")
	ErrOutOfRange       = errors.New("out of range")
)

const (
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

func (f *GCSFile) ReadAll() (file.RowReader, error) {
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
func (f *GCSFile) createJSONReader(location string) (file.RowReader, error) {
	status := statusErr

	defer f.sendOperationStats(&FileLog{Operation: "JSON READER", Location: location, Status: &status}, time.Now())

	buffer, err := io.ReadAll(f.body)
	if err != nil {
		f.logger.Errorf("ReadAll Failed: Unable to read json file: %v", err)
		return nil, err
	}

	reader := bytes.NewReader(buffer)

	decoder := json.NewDecoder(reader)

	// Peek the first JSON token to determine the type
	// Note: This results in offset to move ahead, making it necessary to
	// decode again if we are decoding a json object instead of array
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
func (f *GCSFile) createTextCSVReader(location string) (file.RowReader, error) {
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

// Scan scans the next line from the text file into the provided pointer to string.
func (f *textReader) Scan(i any) error {

	if val, ok := i.(*string); ok {
		*val = f.scanner.Text()
		return nil
	}

	return errStringNotPointer
}

func (g *GCSFile) Name() string {
	bucketName := getBucketName(g.name)

	g.sendOperationStats(&FileLog{
		Operation: "GET NAME",
		Location:  getLocation(bucketName),
	}, time.Now())

	return g.name
}

func (g *GCSFile) Size() int64 {
	bucketName := getBucketName(g.name)

	g.sendOperationStats(&FileLog{
		Operation: "FILE/DIR SIZE",
		Location:  getLocation(bucketName),
	}, time.Now())

	return g.size
}

func (g *GCSFile) ModTime() time.Time {
	bucketName := getBucketName(g.name)

	g.sendOperationStats(&FileLog{
		Operation: "LAST MODIFIED",
		Location:  getLocation(bucketName),
	}, time.Now())

	return g.lastModified
}

func (g *GCSFile) Mode() fs.FileMode {
	bucketName := getBucketName(g.name)

	g.sendOperationStats(&FileLog{
		Operation: "MODE",
		Location:  getLocation(bucketName),
	}, time.Now())

	if g.isDir {
		return fs.ModeDir
	}
	return 0
}

func (g *GCSFile) IsDir() bool {
	bucketName := getBucketName(g.name)

	g.sendOperationStats(&FileLog{
		Operation: "IS DIR",
		Location:  getLocation(bucketName),
	}, time.Now())

	return g.isDir || g.contentType == "application/x-directory"
}

func (g *GCSFile) Sys() interface{} {
	bucketName := getBucketName(g.name)

	g.sendOperationStats(&FileLog{
		Operation: "SYS",
		Location:  getLocation(bucketName),
	}, time.Now())

	return nil
}
