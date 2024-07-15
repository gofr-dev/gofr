package file

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"gofr.dev/pkg/gofr/datasource"
)

var errNotPointer = errors.New("input should be a pointer to a string")

type file struct {
	*os.File
	logger datasource.Logger
}

type textReader struct {
	scanner *bufio.Scanner
	logger  datasource.Logger
}

type jsonReader struct {
	decoder *json.Decoder
	token   json.Token
}

// ReadAll reads either json, csv or text files, file with multiple rows, objects or single object can be read
// in the same way.
// File format is decided based on the extension
// JSON files are read in struct, while CSV files are read in pointer to string.
//
// newCsvFile, _ = fileStore.Open("file.csv")
// reader := newCsvFile.ReadAll()
//
// Reading JSON files
//
//	for reader.Next() {
//		var u User
//		reader.Scan(&u)
//	}
//
// Reading CSV files
//
//	for reader.Next() {
//		    var content string
//		    reader.Scan(&u)
//	}
func (f file) ReadAll() (datasource.RowReader, error) {
	if strings.HasSuffix(f.File.Name(), ".json") {
		return f.createJSONReader()
	}

	return f.createTextCSVReader(), nil
}

// Factory method to create the appropriate JSON reader.
func (f file) createJSONReader() (datasource.RowReader, error) {
	decoder := json.NewDecoder(f.File)

	token, err := f.peekJSONToken(decoder)
	if err != nil {
		f.logger.Errorf("failed to decode JSON token %v", err)
		return nil, err
	}

	if d, ok := token.(json.Delim); ok && d == '[' {
		// JSON array
		return &jsonReader{decoder: decoder, token: token}, nil
	}

	// JSON object
	return f.createJSONObjectReader()
}

// Peek the first JSON token to determine its type.
func (file) peekJSONToken(decoder *json.Decoder) (json.Token, error) {
	newDecoder := *decoder

	token, err := newDecoder.Token()
	if err != nil {
		return nil, err
	}

	return token, nil
}

// Create a JSON reader for a JSON object.
func (f file) createJSONObjectReader() (datasource.RowReader, error) {
	name := f.File.Name()

	if err := f.File.Close(); err != nil {
		f.logger.Errorf("failed to close JSON file for reading as object %v", err)
		return nil, err
	}

	newFile, err := os.Open(name)
	if err != nil {
		f.logger.Errorf("failed to open JSON file for reading as object %v", err)
		return nil, err
	}

	decoder := json.NewDecoder(newFile)

	return &jsonReader{decoder: decoder}, nil
}

func (f file) createTextCSVReader() datasource.RowReader {
	return &textReader{
		scanner: bufio.NewScanner(f.File),
		logger:  f.logger,
	}
}

// Next checks if there is next json object available otherwise returns false.
func (j jsonReader) Next() bool {
	return j.decoder.More()
}

// Scan binds the data to provided struct.
func (j jsonReader) Scan(i interface{}) error {
	return j.decoder.Decode(&i)
}

// Next checks if there is data available in next line otherwise returns false.
func (f textReader) Next() bool {
	return f.scanner.Scan()
}

// Scan binds the line to provided pointer to string.
func (f textReader) Scan(i interface{}) error {
	switch target := i.(type) {
	case *string:
		*target = f.scanner.Text()
		return nil
	default:
		return errNotPointer
	}
}
