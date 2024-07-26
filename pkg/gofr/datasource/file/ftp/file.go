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
const JsonBufferLength = 1024

// errNotPointer is returned when Read method is called with a non-pointer argument.
var (
	errNotPointer  = errors.New("input should be a pointer to a string")
	errReadingFile = errors.New("error reading file")
)

// ftpFile represents a file on an FTP server.
type ftpFile struct {
	response ftpResponse
	path     string
	conn     ServerConn
	name     string
	offset   int64
	logger   Logger
}

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
func (f *ftpFile) ReadAll() (RowReader, error) {
	if strings.HasSuffix(f.Name(), ".json") {
		return f.createJSONReader()
	}

	return f.createTextCSVReader()
}

// createJSONReader creates a JSON reader for JSON files.
func (f *ftpFile) createJSONReader() (RowReader, error) {
	buffer := make([]byte, JsonBufferLength)

	n, err := f.Read(buffer)
	if err != nil && err != io.EOF {
		f.logger.Errorf("failed to read json file: %v", err)
		return nil, err
	}

	if n == JsonBufferLength && buffer[n-1] != 10 && buffer[n-1] != 93 {
		m := bytes.LastIndexByte(buffer, byte('\n'))
		buffer = append(buffer[:m-1], byte(']'))
		f.offset -= int64(n - m - 1)
		n = m
	}

	buffer = buffer[:n]

	reader := bytes.NewReader(buffer)

	decoder := json.NewDecoder(reader)

	// Peek the first JSON token to determine the type
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	// Check if the JSON is an array
	if d, ok := token.(json.Delim); ok && d == '[' {
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

	if err := f.Close(); err != nil {
		return nil, err
	}

	buffer := make([]byte, BufferLength)

	res, err := f.conn.Retr(name)
	if err != nil {
		return nil, err
	}

	n, err := res.Read(buffer)
	if err != nil {
		return nil, err
	}

	buffer = buffer[:n]

	reader := bytes.NewReader(buffer)

	decoder := json.NewDecoder(reader)

	return &jsonReader{decoder: decoder}, nil
}

// createTextCSVReader creates a text reader for reading text files.
func (f *ftpFile) createTextCSVReader() (RowReader, error) {
	buffer := make([]byte, BufferLength)

	n, err := f.Read(buffer)
	if err != nil && err != io.EOF {
		f.logger.Errorf("failed to read text file: %v", err)
		return nil, err
	}

	if n == BufferLength && buffer[n-1] != 10 {
		m := bytes.LastIndexByte(buffer, byte('\n'))

		f.offset -= int64(n - m - 1)
		n = m
	}

	buffer = buffer[:n]

	reader := bytes.NewReader(buffer)

	return &textReader{
		scanner: bufio.NewScanner(reader),
		logger:  f.logger,
	}, err
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
	return f.response.Close()
}

// Name returns the name of the file.
func (f *ftpFile) Name() string {
	return f.name
}

// Read reads data from the FTP file into the provided byte slice.
func (f *ftpFile) Read(p []byte) (n int, err error) {
	r, err := f.conn.RetrFrom(f.path, uint64(f.offset))
	if err != nil {
		return 0, err
	}

	b, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}

	r.Close()

	copy(p, b)

	readbytes := len(p)

	if len(b) < len(p) {
		f.offset += int64(len(b))
		readbytes = len(b)
		return len(b), io.EOF
	}

	f.logger.Logf("Read %v bytes from %v", readbytes, f.path)

	f.offset += int64(readbytes)

	return len(p), nil
}

// ReadAt reads data from the FTP file starting at the specified offset.
func (f *ftpFile) ReadAt(p []byte, off int64) (n int, err error) {
	resp, err := f.conn.RetrFrom(f.path, uint64(off))
	if err != nil {
		return 0, err
	}

	b, err := io.ReadAll(resp)
	if err != nil {
		return 0, err
	}

	copy(p, b)

	resp.Close()

	f.logger.Logf("Read %v bytes from %v at offset of %v", len(p), f.path, off)

	return len(b), nil
}

func (f *ftpFile) Check(whence int, offset, length int64) (int64, error) {
	switch whence {
	case io.SeekStart:
		if offset < 0 || offset > length {
			return 0, ErrOutOfRange
		}

		return offset, nil

	case io.SeekEnd:
		if offset > 0 || offset < -length {
			return 0, ErrOutOfRange
		}

		return length + offset, nil

	case io.SeekCurrent:
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

	r, err = f.conn.Retr(f.path)
	if err != nil {
		return 0, err
	}

	p, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}

	err = r.Close()
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

	err = f.conn.StorFrom(f.path, reader, uint64(f.offset))
	if err != nil {
		return 0, errors.New("failed to write file")
	}

	f.offset += int64(len(p))

	f.logger.Logf("Wrote %v bytes to %v", len(p), f.path)

	return len(p), nil
}

// WriteAt writes data to the FTP file starting at the specified offset.
func (f *ftpFile) WriteAt(p []byte, off int64) (n int, err error) {
	reader := bytes.NewReader(p)

	err = f.conn.StorFrom(f.path, reader, uint64(off))
	if err != nil {
		return 0, fmt.Errorf("failed to write at offset: %v", off)
	}

	f.logger.Logf("Wrote %v bytes to %v at %v offset", len(p), f.path, off)

	return len(p), nil
}
