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
	bucketName := file.GetBucketName(f.name)
	location := path.Join(bucketName, f.name)

	var msg string

	st := file.StatusError
	startTime := time.Now()

	defer f.observe(file.OpReadAll, startTime, &st, &msg)

	if strings.HasSuffix(f.Name(), ".json") {
		reader, err := f.createJSONReader(location)
		if err == nil {
			st = file.StatusSuccess
			msg = "JSON reader created successfully"
		} else {
			msg = err.Error()
		}

		return reader, err
	}

	reader, err := f.createTextCSVReader(location)
	if err == nil {
		st = file.StatusSuccess
		msg = "Text reader created successfully"
	} else {
		msg = err.Error()
	}

	return reader, err
}

// createJSONReader creates a JSON reader for JSON files.
func (f *File) createJSONReader(location string) (file.RowReader, error) {
	var msg string

	st := file.StatusError
	startTime := time.Now()

	defer f.observe(file.OpJSONReader, startTime, &st, &msg)

	buffer, err := io.ReadAll(f.body)
	if err != nil {
		f.logger.Errorf("failed to read JSON body from location %s: %v", location, err)

		msg = err.Error()

		return nil, err
	}

	reader := bytes.NewReader(buffer)

	decoder := json.NewDecoder(reader)

	token, err := decoder.Token()
	if err != nil {
		f.logger.Errorf("Error decoding token: %v", err)
		msg = err.Error()

		return nil, err
	}

	if d, ok := token.(json.Delim); ok && d == '[' {
		st = file.StatusSuccess
		msg = "JSON array reader created successfully"

		return &jsonReader{decoder: decoder, token: token}, err
	}

	// Reading JSON object
	decoder = json.NewDecoder(reader)
	st = file.StatusSuccess
	msg = "JSON object reader created successfully"

	return &jsonReader{decoder: decoder}, nil
}

// createTextCSVReader creates a text reader for reading text files.
func (f *File) createTextCSVReader(location string) (file.RowReader, error) {
	var msg string

	st := file.StatusError
	startTime := time.Now()

	defer f.observe(file.OpTextCSVReader, startTime, &st, &msg)

	buffer, err := io.ReadAll(f.body)
	if err != nil {
		f.logger.Errorf("failed to read text/csv body from location %s: %v", location, err)
		msg = err.Error()

		return nil, err
	}

	reader := bytes.NewReader(buffer)
	st = file.StatusSuccess
	msg = "Text/CSV reader created successfully"

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
	st := file.StatusSuccess
	msg := "Name retrieved"
	startTime := time.Now()

	defer f.observe(file.OpGetName, startTime, &st, &msg)

	return f.name
}

func (f *File) Size() int64 {
	st := file.StatusSuccess
	msg := "Size retrieved"
	startTime := time.Now()

	defer f.observe(file.OpFileSize, startTime, &st, &msg)

	return f.size
}

func (f *File) ModTime() time.Time {
	st := file.StatusSuccess
	msg := "ModTime retrieved"
	startTime := time.Now()

	defer f.observe(file.OpLastMod, startTime, &st, &msg)

	return f.lastModified
}

func (f *File) Mode() fs.FileMode {
	st := file.StatusSuccess
	msg := "Mode retrieved"
	startTime := time.Now()

	defer f.observe(file.OpMode, startTime, &st, &msg)

	if f.isDir {
		return fs.ModeDir
	}

	return 0
}

func (f *File) IsDir() bool {
	st := file.StatusSuccess
	msg := "IsDir checked"
	startTime := time.Now()

	defer f.observe(file.OpIsDir, startTime, &st, &msg)

	return f.isDir || f.contentType == "application/x-directory"
}

func (f *File) Sys() any {
	st := file.StatusSuccess
	msg := "Sys info retrieved"
	startTime := time.Now()

	defer f.observe(file.OpSys, startTime, &st, &msg)

	return nil
}
