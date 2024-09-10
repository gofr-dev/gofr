package s3

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path"
	"path/filepath"
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

// ReadAll reads either JSON or text files based on file extension and returns a corresponding RowReader.
func (f *s3file) ReadAll() (file.RowReader, error) {
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
func (f *s3file) createJSONReader(location string) (file.RowReader, error) {
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
func (f *s3file) createTextCSVReader(location string) (file.RowReader, error) {
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

	return errStringNotPointer
}
