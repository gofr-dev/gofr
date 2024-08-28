package ftp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"

	file_interface "gofr.dev/pkg/gofr/datasource/file"
)

var (
	// errNotPointer is returned when Read method is called with a non-pointer argument.
	errNotPointer = errors.New("input should be a pointer to a string")
	ErrOutOfRange = errors.New("out of range")
)

// file represents a file on an FTP server.
type file struct {
	response  ftpResponse
	path      string
	entryType ftp.EntryType
	modTime   time.Time
	conn      serverConn
	name      string
	offset    int64
	logger    Logger
	metrics   Metrics
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
func (f *file) ReadAll() (file_interface.RowReader, error) {
	defer f.sendOperationStats(&FileLog{Operation: "ReadAll", Location: f.path}, time.Now())

	if strings.HasSuffix(f.Name(), ".json") {
		return f.createJSONReader()
	}

	return f.createTextCSVReader()
}

// createJSONReader creates a JSON reader for JSON files.
func (f *file) createJSONReader() (file_interface.RowReader, error) {
	status := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "JSON Reader", Location: f.path, Status: &status}, time.Now())

	res, err := f.conn.Retr(f.path)
	if err != nil {
		f.logger.Errorf("ReadAll failed: Unable to retrieve json file: %v", err)
		return nil, err
	}

	defer res.Close()

	buffer, err := io.ReadAll(res)
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
		return nil, err
	}

	if d, ok := token.(json.Delim); ok && d == '[' {
		status = "SUCCESS"

		return &jsonReader{decoder: decoder, token: token}, err
	}

	// Reading JSON object
	decoder = json.NewDecoder(reader)
	status = "SUCCESS"

	return &jsonReader{decoder: decoder}, nil
}

// createTextCSVReader creates a text reader for reading text files.
func (f *file) createTextCSVReader() (file_interface.RowReader, error) {
	status := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "Text/CSV Reader", Location: f.path, Status: &status}, time.Now())

	res, err := f.conn.Retr(f.path)
	if err != nil {
		f.logger.Errorf("ReadAll failed: Unable to retrieve text file: %v", err)
		return nil, err
	}

	defer res.Close()

	buffer, err := io.ReadAll(res)
	if err != nil {
		f.logger.Errorf("ReadAll failed: Unable to read text file: %v", err)
		return nil, err
	}

	reader := bytes.NewReader(buffer)
	status = "SUCCESS"

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
	var status string

	defer f.sendOperationStats(&FileLog{Operation: "Close", Location: f.path, Status: &status}, time.Now())

	err := f.response.Close()
	if err != nil {
		status = "ERROR"

		return err
	}

	status = "SUCCESS"

	return nil
}

// Name returns the name of the file.
func (f *file) Name() string {
	defer f.sendOperationStats(&FileLog{Operation: "Get Name", Location: f.path}, time.Now())

	return f.name
}

// Size returns the size of the file.
func (f *file) Size() int64 {
	var msg string

	status := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "Size", Location: f.path, Status: &status, Message: &msg}, time.Now())

	size, err := f.conn.FileSize(f.name)
	if err != nil {
		f.logger.Errorf("Size operation failed: %v", err)
	}

	return size
}

// Mode checks the FileMode. FTP server doesn't support file modes.
// This method is to comply with the generalized FileInfo interface.
func (f *file) Mode() os.FileMode {
	f.sendOperationStats(&FileLog{Operation: "Mode", Location: f.path}, time.Now())
	return os.ModePerm
}

// IsDir checks, if the file is a directory or not.
// Note: IsDir must be used post Stat/ReadDir methods of fileSystem only.
func (f *file) IsDir() bool {
	defer f.sendOperationStats(&FileLog{Operation: "IsDir", Location: f.path}, time.Now())
	return f.entryType == ftp.EntryTypeFolder
}

// ModTime returns the last time the file/directory was modified.
// Note: ModTime must be used post Stat/ReadDir methods of fileSystem only.
func (f *file) ModTime() time.Time {
	defer f.sendOperationStats(&FileLog{Operation: "ModTime", Location: f.path}, time.Now())

	t, err := f.conn.GetTime(f.path)
	if err != nil {
		f.logger.Errorf("ModTime operation failed: %v", err)
		return time.Time{}
	}

	return t
}

// Read reads data from the FTP file into the provided byte slice and updates the file offset.
func (f *file) Read(p []byte) (n int, err error) {
	var msg string

	status := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "Read", Location: f.path, Status: &status, Message: &msg}, time.Now())

	r, err := f.conn.RetrFrom(f.path, uint64(f.offset))
	if err != nil {
		f.logger.Errorf("Read failed: Failed to open file with path %q : %v", f.path, err)
		return 0, err
	}

	defer r.Close()

	n, err = r.Read(p)

	f.offset += int64(n)

	if err != nil && !errors.Is(err, io.EOF) {
		f.logger.Errorf("Read failed: Failed to read from %q : %v", f.path, err)
		return n, err
	}

	status = "SUCCESS"
	msg = fmt.Sprintf("Read %v bytes from file with path %q", n, f.path)

	return n, err
}

// ReadAt reads data from the FTP file starting at the specified offset.
func (f *file) ReadAt(p []byte, off int64) (n int, err error) {
	var msg string

	status := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "ReadAt", Location: f.path, Status: &status, Message: &msg}, time.Now())

	resp, err := f.conn.RetrFrom(f.path, uint64(off))
	if err != nil {
		f.logger.Errorf("ReadAt failed: Error opening file with path %q at %v offset : %v", f.path, off, err)
		return 0, err
	}

	defer resp.Close()

	n, err = resp.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		f.logger.Errorf("ReadAt failed: Error reading file on path %q at %v offset : %v", off, f.path, err)
		return 0, err
	}

	status = "SUCCESS"
	msg = fmt.Sprintf("Read %v bytes from file with path %q at offset of %v", n, f.path, off)

	return n, err
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

// Seek sets the offset for the next Read/ Write operations.
func (f *file) Seek(offset int64, whence int) (int64, error) {
	var msg string

	status := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "Seek", Location: f.path, Status: &status, Message: &msg}, time.Now())

	n, err := f.conn.FileSize(f.path)
	if err != nil {
		f.logger.Errorf("Seek failed, error: %v", err)
		return 0, err
	}

	res, err := f.check(whence, offset, n)
	if err != nil {
		f.logger.Errorf("Seek failed, error: %v", err)
		return 0, err
	}

	status = "SUCCESS"
	msg = fmt.Sprintf("Offset set to %v for file at path %q", res, f.path)

	return res, nil
}

// Write writes data to the FTP file.
func (f *file) Write(p []byte) (n int, err error) {
	var msg string

	status := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "Write", Location: f.path, Status: &status, Message: &msg}, time.Now())

	reader := bytes.NewReader(p)

	err = f.conn.StorFrom(f.path, reader, uint64(f.offset))
	if err != nil {
		f.logger.Errorf("Write failed, error: %v", err)
		return 0, err
	}

	f.offset += int64(len(p))

	mt := f.ModTime()
	if !mt.IsZero() {
		f.modTime = mt
	}

	status = "SUCCESS"
	msg = fmt.Sprintf("Wrote %v bytes to file at path %q", len(p), f.path)

	return len(p), nil
}

// WriteAt writes data to the FTP file starting at the specified offset.
func (f *file) WriteAt(p []byte, off int64) (n int, err error) {
	var msg string

	status := "ERROR"

	defer f.sendOperationStats(&FileLog{Operation: "WriteAt", Location: f.path, Status: &status, Message: &msg}, time.Now())

	reader := bytes.NewReader(p)

	err = f.conn.StorFrom(f.path, reader, uint64(off))
	if err != nil {
		f.logger.Errorf("WriteAt failed. Error writing in file with path %q at %v offset : %v", f.path, off, err)
		return 0, err
	}

	mt := f.ModTime()
	if !mt.IsZero() {
		f.modTime = mt
	}

	msg = fmt.Sprintf("Wrote %v bytes to file with path %q at offset of %v", len(p), f.path, off)
	status = "SUCCESS"

	return len(p), nil
}

func (f *file) sendOperationStats(fl *FileLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	fl.Duration = duration

	f.logger.Debug(fl)

	f.metrics.RecordHistogram(context.Background(), "app_ftp_stats", float64(duration),
		"type", fl.Operation, "status", *fl.Status)
}
