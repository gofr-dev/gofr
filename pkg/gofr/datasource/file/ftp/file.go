package ftp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

var (
	// errNotPointer is returned when Read method is called with a non-pointer argument.
	errNotPointer = errors.New("input should be a pointer to a string")
	ErrOutOfRange = errors.New("out of range")
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
	size, err := f.conn.FileSize(f.path)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, size)

	_, readerError := f.Read(buffer)

	if readerError != nil && !errors.Is(readerError, io.EOF) {
		f.logger.Errorf("ReadAll Failed Failed to read json file: %v", readerError)
		return nil, readerError
	}

	reader := bytes.NewReader(buffer)

	decoder := json.NewDecoder(reader)

	// Peek the first JSON token to determine the type
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	if d, ok := token.(json.Delim); ok && d == '[' {
		return &jsonReader{decoder: decoder, token: token}, readerError
	}

	// JSON object
	return f.createJSONObjectReader(reader)
}

func (_ *ftpFile) createJSONObjectReader(reader *bytes.Reader) (RowReader, error) {
	decoder := json.NewDecoder(reader)

	return &jsonReader{decoder: decoder}, nil
}

// createTextCSVReader creates a text reader for reading text files.
func (f *ftpFile) createTextCSVReader() (RowReader, error) {
	size, err := f.conn.FileSize(f.path)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, size)

	_, err = f.Read(buffer)
	if err != nil {
		f.logger.Errorf("ReadAll Failed Failed to read text file: %v", err)
		return nil, err
	}

	reader := bytes.NewReader(buffer)

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

// Read reads data from the FTP file into the provided byte slice and updates the file offset.
func (f *ftpFile) Read(p []byte) (n int, err error) {
	r, err := f.conn.RetrFrom(f.path, uint64(f.offset))
	if err != nil {
		f.logger.Errorf("Read Failed Failed to open file with path %q : %v", f.path, err)
		return 0, err
	}

	n, err = r.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		f.logger.Errorf("Read Failed Failed to read from %q : %v", f.path, err)
		return 0, err
	}

	r.Close()

	f.offset += int64(n)

	f.logger.Logf("Read Success Read %v bytes from %q", n, f.path)

	if n < len(p) {
		return n, io.EOF
	}

	return n, nil
}

// ReadAt reads data from the FTP file starting at the specified offset.
func (f *ftpFile) ReadAt(p []byte, off int64) (n int, err error) {
	resp, err := f.conn.RetrFrom(f.path, uint64(off))
	if err != nil {
		f.logger.Errorf("ReadAt Failed Error opening file with path %q at %v offset : %v", f.path, off, err)
		return 0, err
	}

	n, err = resp.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		f.logger.Errorf("ReadAt Failed Error reading file with path %q at %v offset : %v", f.path, off, err)
		return 0, err
	}

	resp.Close()

	f.logger.Logf("ReadAt Success Read %v bytes from %q at offset of %v", n, f.path, off)

	if n < len(p) {
		return n, io.EOF
	}

	return n, nil
}

func (f *ftpFile) check(whence int, offset, length int64) (int64, error) {
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
	n, err := f.conn.FileSize(f.path)
	if err != nil {
		f.logger.Errorf("Seek Failed Error : %v", err)
		return 0, err
	}

	res, err := f.check(whence, offset, n)
	if err != nil {
		f.logger.Errorf("Seek Failed Error : %v", err)
		return 0, err
	}

	f.logger.Logf("Seek Success Offset at whence %v : %v", whence, res)

	return res, err
}

// Write writes data to the FTP file.
func (f *ftpFile) Write(p []byte) (n int, err error) {
	reader := bytes.NewReader(p)

	err = f.conn.StorFrom(f.path, reader, uint64(f.offset))
	if err != nil {
		f.logger.Errorf("Write Failed Error : %v", err)
		return 0, err
	}

	f.offset += int64(len(p))

	f.logger.Logf("Write Success Wrote %v bytes to %v", len(p), f.path)

	return len(p), nil
}

// WriteAt writes data to the FTP file starting at the specified offset.
func (f *ftpFile) WriteAt(p []byte, off int64) (n int, err error) {
	reader := bytes.NewReader(p)

	err = f.conn.StorFrom(f.path, reader, uint64(off))
	if err != nil {
		f.logger.Errorf("WriteAt Failed Error writing in file with path %q at %v offset : %v", f.path, off, err)
		return 0, err
	}

	f.logger.Logf("WriteAt Success Wrote %v bytes to %q at %v offset", len(p), f.path, off)

	return len(p), nil
}
