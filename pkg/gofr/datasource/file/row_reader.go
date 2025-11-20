package file

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"gofr.dev/pkg/gofr/datasource"
)

var (
	errNotPointer  = errors.New("input should be a pointer to a string")
	errInvalidJSON = errors.New("invalid JSON")
)

// textReader reads text/CSV files line by line.
type textReader struct {
	scanner *bufio.Scanner
	logger  datasource.Logger
}

// jsonReader reads JSON files (array or single object).
type jsonReader struct {
	decoder  *json.Decoder
	isArray  bool
	consumed bool
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
	// Read entire payload so we can validate and then create a fresh decoder.
	b, err := io.ReadAll(r)
	if err != nil {
		if logger != nil {
			logger.Errorf("failed to read JSON input: %v", err)
		}

		return nil, err
	}

	trimmed := bytes.TrimLeft(b, " \t\r\n")
	if len(trimmed) == 0 {
		return nil, io.EOF
	}

	// Validate JSON to detect invalid input early.
	if !json.Valid(trimmed) {
		if logger != nil {
			logger.Errorf("invalid JSON input")
		}

		return nil, errInvalidJSON
	}

	isArray := trimmed[0] == '['
	decoder := json.NewDecoder(bytes.NewReader(trimmed))

	// If array, consume the opening '[' so decoder.More() works properly.
	if isArray {
		_, err := decoder.Token()
		if err != nil {
			if logger != nil {
				logger.Errorf("failed to read JSON array token: %v", err)
			}

			return nil, err
		}
	}

	return &jsonReader{
		decoder:  decoder,
		isArray:  isArray,
		consumed: false,
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
