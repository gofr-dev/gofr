package ftp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"gofr.dev/pkg/gofr/container"
)

var (
	// errNotPointer is returned when Read method is called with a non-pointer argument.
	errNotPointer = errors.New("input should be a pointer to a string")
	ErrOutOfRange = errors.New("out of range")
)

// ftpFile represents a file on an FTP server.
type file struct {
	response ftpResponse
	path     string
	conn     ServerConn
	name     string
	offset   int64
	logger   Logger
	metrics  Metrics
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
func (f *file) ReadAll() (container.RowReader, error) {
	defer f.postProcess(&FileLog{Operation: "ReadAll", Location: f.path}, time.Now())

	if strings.HasSuffix(f.Name(), ".json") {
		return f.createJSONReader()
	}

	return f.createTextCSVReader()
}

// createJSONReader creates a JSON reader for JSON files.
func (f *file) createJSONReader() (container.RowReader, error) {
	defer f.postProcess(&FileLog{Operation: "JSON Reader", Location: f.path}, time.Now())

	res, err := f.conn.Retr(f.path)
	if err != nil {
		f.logger.Errorf("ReadAll failed. Unable to retrieve json file: %v", err)
		return nil, err
	}

	defer res.Close()

	buffer, err := io.ReadAll(res)
	if err != nil {
		f.logger.Errorf("ReadAll Failed. Unable to read json file: %v", err)
		return nil, err
	}

	reader := bytes.NewReader(buffer)

	decoder := json.NewDecoder(reader)

	// Peek the first JSON token to determine the type
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	if d, ok := token.(json.Delim); ok && d == '[' {
		return &jsonReader{decoder: decoder, token: token}, err
	}

	// Reading JSON object
	decoder = json.NewDecoder(reader)

	return &jsonReader{decoder: decoder}, nil
}

// createTextCSVReader creates a text reader for reading text files.
func (f *file) createTextCSVReader() (container.RowReader, error) {
	defer f.postProcess(&FileLog{Operation: "Text/CSV Reader", Location: f.path}, time.Now())

	res, err := f.conn.Retr(f.path)
	if err != nil {
		f.logger.Errorf("ReadAll failed. Unable to retrieve text file: %v", err)
		return nil, err
	}

	defer res.Close()

	buffer, err := io.ReadAll(res)
	if err != nil {
		f.logger.Errorf("ReadAll failed. Unable to read text file: %v", err)
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
	if val, ok := i.(*string); ok {
		*val = f.scanner.Text()
		return nil
	}

	return errNotPointer
}

// Close closes the FTP file connection.
func (f *file) Close() error {
	defer f.postProcess(&FileLog{Operation: "Close", Location: f.path}, time.Now())

	return f.response.Close()
}

// Name returns the name of the file.
func (f *file) Name() string {
	defer f.postProcess(&FileLog{Operation: "Get Name", Location: f.path}, time.Now())

	return f.name
}

// Read reads data from the FTP file into the provided byte slice and updates the file offset.
func (f *file) Read(p []byte) (n int, err error) {
	defer f.postProcess(&FileLog{Operation: "Read", Location: f.path}, time.Now())

	r, err := f.conn.RetrFrom(f.path, uint64(f.offset))
	if err != nil {
		f.logger.Errorf("Read failed : Failed to open file with path %q : %v", f.path, err)
		return 0, err
	}

	defer r.Close()

	n, err = r.Read(p)

	f.offset += int64(n)

	if err != nil && !errors.Is(err, io.EOF) {
		f.logger.Errorf("Read failed : Failed to read from %q : %v", f.path, err)
		return n, err
	}

	f.logger.Logf("Read successful : Read %v bytes from %q", n, f.path)

	if errors.Is(err, io.EOF) {
		return n, io.EOF
	}

	return n, nil
}

// ReadAt reads data from the FTP file starting at the specified offset.
func (f *file) ReadAt(p []byte, off int64) (n int, err error) {
	defer f.postProcess(&FileLog{Operation: "ReadAt", Location: f.path}, time.Now())

	resp, err := f.conn.RetrFrom(f.path, uint64(off))
	if err != nil {
		f.logger.Errorf("ReadAt failed. Error opening file with path %q at %v offset : %v", f.path, off, err)
		return 0, err
	}

	defer resp.Close()

	n, err = resp.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		f.logger.Errorf("ReadAt failed. Error reading file with path %q at %v offset : %v", f.path, off, err)
		return 0, err
	}

	f.logger.Logf("ReadAt successful. Read %v bytes from %q at offset of %v", n, f.path, off)

	if n < len(p) {
		return n, io.EOF
	}

	return n, nil
}

func (f *file) check(whence int, offset, length int64) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekEnd:
		offset += length
	case io.SeekCurrent:
		offset += f.offset
	default:
		return 0, os.ErrInvalid
	}

	if offset < 0 || offset > length {
		return 0, ErrOutOfRange
	}

	f.offset = offset

	return f.offset, nil
}

// Seek sets the offset for the next Read or ReadAt operation.
func (f *file) Seek(offset int64, whence int) (int64, error) {
	defer f.postProcess(&FileLog{Operation: "Seek", Location: f.path}, time.Now())

	n, err := f.conn.FileSize(f.path)
	if err != nil {
		f.logger.Errorf("Seek failed. Error : %v", err)
		return 0, err
	}

	res, err := f.check(whence, offset, n)
	if err != nil {
		f.logger.Errorf("Seek failed. Error : %v", err)
		return 0, err
	}

	f.logger.Logf("Seek successful. Offset at whence %v : %v", whence, res)

	return res, nil
}

// Write writes data to the FTP file.
func (f *file) Write(p []byte) (n int, err error) {
	defer f.postProcess(&FileLog{Operation: "Write", Location: f.path}, time.Now())

	reader := bytes.NewReader(p)

	err = f.conn.StorFrom(f.path, reader, uint64(f.offset))
	if err != nil {
		f.logger.Errorf("Write failed. Error : %v", err)
		return 0, err
	}

	f.offset += int64(len(p))

	f.logger.Logf("Write successful. Wrote %v bytes to %v", len(p), f.path)

	return len(p), nil
}

// WriteAt writes data to the FTP file starting at the specified offset.
func (f *file) WriteAt(p []byte, off int64) (n int, err error) {
	defer f.postProcess(&FileLog{Operation: "WriteAt", Location: f.path}, time.Now())

	reader := bytes.NewReader(p)

	err = f.conn.StorFrom(f.path, reader, uint64(off))
	if err != nil {
		f.logger.Errorf("WriteAt failed. Error writing in file with path %q at %v offset : %v", f.path, off, err)
		return 0, err
	}

	f.logger.Logf("WriteAt success. Wrote %v bytes to %q at %v offset", len(p), f.path, off)

	return len(p), nil
}

func (f *file) postProcess(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	fl.Duration = duration

	f.logger.Debugf("%v", fl)
}
