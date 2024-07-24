package ftp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const BufferLength = 1024

// errNotPointer is returned when Read method is called with a non-pointer argument.
var errNotPointer = errors.New("input should be a pointer to a string")

// ftpFile represents a file on an FTP server.
type ftpFile struct {
	response ftpResponse // FTP response object
	path     string      // Path of the file on the server
	conn     ServerConn  // FTP server connection
	name     string      // Name of the file
	offset   int64       // Offset for Seek operations
	logger   Logger      // Logger interface for logging
}

// textReader implements datasource.RowReader for reading text files.
type textReader struct {
	scanner *bufio.Scanner // Scanner for reading lines
	logger  Logger         // Logger interface for logging
}

// jsonReader implements datasource.RowReader for reading JSON files.
type jsonReader struct {
	decoder *json.Decoder // JSON decoder
	token   json.Token    // Current JSON token
}

// ReadAll reads either JSON or text files based on file extension and returns a corresponding RowReader.
func (f *ftpFile) ReadAll() (RowReader, error) {
	if strings.HasSuffix(f.Name(), ".json") {
		return f.createJSONReader()
	}

	return f.createTextCSVReader(), nil
}

// createJSONReader creates a JSON reader for JSON files.
func (f *ftpFile) createJSONReader() (RowReader, error) {
	// Initializing a buffer and reading the response
	emptyBuffer := make([]byte, BufferLength)

	n, err := f.response.Read(emptyBuffer)
	if err != nil {
		return nil, err
	}

	emptyBuffer = emptyBuffer[:n]

	reader := bytes.NewReader(emptyBuffer)

	// Create a JSON decoder
	decoder := json.NewDecoder(reader)

	// Peek the first JSON token to determine the type
	token, err := f.peekJSONToken(decoder)
	if err != nil {
		return nil, err
	}

	// Check if the JSON is an array
	if d, ok := token.(json.Delim); ok && d == '[' {
		// JSON array
		return &jsonReader{decoder: decoder, token: token}, nil
	}

	// else reading the json object
	return f.createJSONObjectReader()
}

// peekJSONToken peeks the first JSON token without advancing the decoder.
func (*ftpFile) peekJSONToken(decoder *json.Decoder) (json.Token, error) {
	newDecoder := *decoder

	return newDecoder.Token()
}

// createJSONObjectReader creates a JSON reader for a JSON object.
func (f *ftpFile) createJSONObjectReader() (RowReader, error) {
	name := f.Name()

	// Close the current file handle
	if err := f.Close(); err != nil {
		return nil, err
	}

	buffer := make([]byte, BufferLength)

	// Retrieve the file as *ftp.Response
	res, err := f.conn.Retr(name)
	if err != nil {
		return nil, err
	}

	// Read the response
	n, err := res.Read(buffer)
	if err != nil {
		return nil, err
	}

	buffer = buffer[:n]

	reader := bytes.NewReader(buffer)

	// Create a JSON decoder
	decoder := json.NewDecoder(reader)

	return &jsonReader{decoder: decoder}, nil
}

// createTextCSVReader creates a text reader for reading text files.
func (f *ftpFile) createTextCSVReader() RowReader {
	// Read the file content into a buffer
	buffer := make([]byte, BufferLength)

	n, err := f.response.Read(buffer)
	if err != nil {
		f.logger.Errorf("failed to read text file: %v", err)
	}

	buffer = buffer[:n]

	reader := bytes.NewReader(buffer)

	return &textReader{
		scanner: bufio.NewScanner(reader),
		logger:  f.logger,
	}
}

// Next checks if there is another JSON object available.
func (j jsonReader) Next() bool {
	return j.decoder.More()
}

// Scan decodes the next JSON object into the provided structure.
func (j jsonReader) Scan(i interface{}) error {
	return j.decoder.Decode(&i)
}

// Next checks if there is another line available in the text file.
func (f textReader) Next() bool {
	return f.scanner.Scan()
}

// Scan scans the next line from the text file into the provided pointer to string.
func (f textReader) Scan(i interface{}) error {
	switch target := i.(type) {
	case *string:
		*target = f.scanner.Text()
		return nil
	default:
		return errNotPointer
	}
}

// Close closes the FTP file connection.
func (f *ftpFile) Close() error {
	// Close the FTP response.
	return f.response.Close()
}

// Name returns the name of the file.
func (f *ftpFile) Name() string {
	return f.name
}

// Read reads data from the FTP file into the provided byte slice.
func (f *ftpFile) Read(p []byte) (n int, err error) {
	// Retrieve the file content from FTP server and read into p
	r, err := f.conn.Retr(f.path)
	if err != nil {
		return 0, err
	}

	b, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}

	copy(p, b)

	f.offset = int64(len(b))

	return len(b), nil
}

// ReadAt reads data from the FTP file starting at the specified offset.
func (f *ftpFile) ReadAt(p []byte, off int64) (n int, err error) {
	// Retrieve file content from the specified offset
	resp, err := f.conn.RetrFrom(f.path, uint64(off))
	if err != nil {
		return 0, err
	}

	b, err := io.ReadAll(resp)
	if err != nil {
		return 0, err
	}

	// Deep Copy
	copy(p, b)

	return len(b), nil
}

func (f *ftpFile) Check(whence int, offset, length int64) (int64, error) {
	switch whence {
	case io.SeekStart:
		// Seek from the beginning of the file
		if offset < 0 || offset > length {
			return 0, ErrOutOfRange
		}

		return offset, nil

	case io.SeekEnd:
		// Seek from the end of the file
		if offset > 0 || offset < -length {
			return 0, ErrOutOfRange
		}

		return length + offset, nil

	case io.SeekCurrent:
		// Seek from the current offset
		if f.offset+offset >= length || f.offset+offset < 0 {
			return 0, ErrOutOfRange
		}

		return f.offset + offset, nil
	default:
	}

	return 0, ErrOutOfRange
}

// Seek sets the offset for the next Read or ReadAt operation.
func (f *ftpFile) Seek(offset int64, whence int) (int64, error) {
	var (
		err error
		r   ftpResponse
	)

	// Handle Seek operation based on whence parameter
	r, err = f.conn.Retr(f.path)
	if err != nil {
		return 0, err
	}

	p, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}

	n := int64(len(p))

	res, err := f.Check(whence, offset, n)

	return res, err
}

// Write writes data to the FTP file.
func (f *ftpFile) Write(p []byte) (n int, err error) {
	reader := bytes.NewReader(p)

	// Write data to the FTP file with offset 0.
	err = f.conn.StorFrom(f.path, reader, uint64(0))
	if err != nil {
		return 0, errors.New("failed to write file")
	}

	f.offset = int64(len(p))

	return len(p), nil
}

// WriteAt writes data to the FTP file starting at the specified offset.
func (f *ftpFile) WriteAt(p []byte, off int64) (n int, err error) {
	reader := bytes.NewReader(p)

	// Write data to the FTP file from the specified offset
	err = f.conn.StorFrom(f.path, reader, uint64(off))
	if err != nil {
		return 0, fmt.Errorf("failed to write at offset: %v", off)
	}

	return len(p), nil
}
