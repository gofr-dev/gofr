package file

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

var (
	// ErrStringNotPointer is returned when Scan is called with a non-pointer string for text readers.
	ErrStringNotPointer = errors.New("input should be a pointer to a string")
)

// cloudTextReader implements RowReader for cloud storage text/CSV files line-by-line.
type cloudTextReader struct {
	scanner *bufio.Scanner
}

// cloudJSONReader implements RowReader for cloud storage JSON files (arrays or objects).
type cloudJSONReader struct {
	decoder *json.Decoder
	isArray bool
}

// NewTextReader creates a RowReader for text/CSV files from cloud storage.
// This is separate from the local filesystem's text reader.
func NewTextReader(r io.Reader) RowReader {
	return &cloudTextReader{
		scanner: bufio.NewScanner(r),
	}
}

// NewJSONReader creates a RowReader for JSON files from cloud storage.
// Handles both JSON arrays and single JSON objects.
func NewJSONReader(r io.Reader) (RowReader, error) {
	// Read entire content into buffer (needed to detect array vs object)
	buffer, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(buffer)
	decoder := json.NewDecoder(reader)

	// Check if JSON is an array
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	if d, ok := token.(json.Delim); ok && d == '[' {
		// JSON array - use streaming decoder
		return &cloudJSONReader{decoder: decoder, isArray: true}, nil
	}

	// JSON object - create new decoder from buffer
	decoder = json.NewDecoder(bytes.NewReader(buffer))

	return &cloudJSONReader{decoder: decoder, isArray: false}, nil
}

// Next checks if there's another line in the text file.
func (t *cloudTextReader) Next() bool {
	return t.scanner.Scan()
}

// Scan reads the next line into the provided string pointer.
func (t *cloudTextReader) Scan(i any) error {
	val, ok := i.(*string)
	if !ok {
		return ErrStringNotPointer
	}

	*val = t.scanner.Text()

	return nil
}

// Next checks if there's another JSON object in the stream.
func (j *cloudJSONReader) Next() bool {
	if j.isArray {
		return j.decoder.More()
	}
	// For single objects, only return true once
	return j.decoder.More()
}

// Scan decodes the next JSON object into the provided structure.
func (j *cloudJSONReader) Scan(i any) error {
	return j.decoder.Decode(i)
}
