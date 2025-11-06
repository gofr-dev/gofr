package file

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"gofr.dev/pkg/gofr/datasource"
)

var errNotPointer = errors.New("input should be a pointer to a string")

// textReader reads text/CSV files line by line.
type textReader struct {
	scanner *bufio.Scanner
	logger  datasource.Logger
}

// jsonReader reads JSON files (array or single object).
type jsonReader struct {
	decoder    *json.Decoder
	firstToken json.Token
	isArray    bool
	consumed   bool
}

// NewTextReader creates a RowReader for text/CSV files.
// Each call to Next() reads one line from the file.
// Scan() must receive a *string pointer to store the line content.
//
// Example:
//
//	reader := file.NewTextReader(f, logger)
//	for reader.Next() {
//	    var line string
//	    reader.Scan(&line)
//	    fmt.Println(line)
//	}
func NewTextReader(r io.Reader, logger datasource.Logger) RowReader {
	return &textReader{
		scanner: bufio.NewScanner(r),
		logger:  logger,
	}
}

// Next checks if there is data available in the next line.
func (t *textReader) Next() bool {
	return t.scanner.Scan()
}

// Scan binds the line to the provided pointer to string.
func (t *textReader) Scan(i any) error {
	target, ok := i.(*string)
	if !ok {
		return errNotPointer
	}

	*target = t.scanner.Text()
	return nil
}

// NewJSONReader creates a RowReader for JSON files (auto-detects array vs object).
func NewJSONReader(r io.Reader, logger datasource.Logger) (RowReader, error) {
	decoder := json.NewDecoder(r)

	// Read first token to determine structure
	token, err := decoder.Token()
	if err != nil {
		if logger != nil {
			logger.Errorf("failed to read JSON structure: %v", err)
		}
		return nil, err
	}

	// Check if it's a JSON array
	isArray := false
	if d, ok := token.(json.Delim); ok && d == '[' {
		isArray = true
	}

	return &jsonReader{
		decoder:    decoder,
		firstToken: token,
		isArray:    isArray,
		consumed:   false,
	}, nil
}

// Next checks if there is a next JSON object available.
func (j *jsonReader) Next() bool {
	// If it's an array, check if more elements exist
	if j.isArray {
		return j.decoder.More()
	}

	// For single object, return true only once
	if !j.consumed {
		return true
	}

	return false
}

// Scan decodes the next JSON object into the provided struct.
func (j *jsonReader) Scan(i any) error {
	// For single objects, mark as consumed after first scan
	if !j.isArray {
		if j.consumed {
			return io.EOF
		}
		j.consumed = true
	}

	// Decode the next object
	return j.decoder.Decode(&i)
}

// createRowReader determines the appropriate reader based on file extension.
// This is used by local filesystem's ReadAll() implementation.
func createRowReader(f *os.File, logger datasource.Logger) (RowReader, error) {
	if strings.HasSuffix(f.Name(), ".json") {
		return NewJSONReader(f, logger)
	}

	// Default to text/CSV reader
	return NewTextReader(f, logger), nil
}
