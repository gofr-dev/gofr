package gcs

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var (
	errorRead = errors.New("read failed")
)

func TestFile_ReadAll_Success_JSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any(),
	).AnyTimes()

	f := &File{
		name:    "data.json",
		body:    io.NopCloser(strings.NewReader(`[{"id":1}]`)),
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	reader, err := f.ReadAll()

	require.NoError(t, err)
	require.NotNil(t, reader)
	_, isJSON := reader.(*jsonReader)
	require.True(t, isJSON, "Expected *jsonReader for .json file")
}

func TestFile_ReadAll_Success_NonJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(), "status", gomock.Any(),
	).AnyTimes()

	testCases := []struct {
		name string
		file string
		data string
	}{
		{
			name: "text file",
			file: "log.txt",
			data: "line1\nline2",
		},
		{
			name: "csv file",
			file: "data.csv",
			data: "name,age\nAlice,30",
		},
		{
			name: "yaml file",
			file: "config.yaml",
			data: "key: value",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{
				name:    tt.file,
				body:    io.NopCloser(strings.NewReader(tt.data)),
				logger:  mockLogger,
				metrics: mockMetrics,
			}

			reader, err := f.ReadAll()

			require.NoError(t, err)
			require.NotNil(t, reader)
			_, isText := reader.(*textReader)
			require.True(t, isText, "Expected *textReader for non-JSON file")
		})
	}
}

type failingReader struct{}

func (failingReader) Read(_ []byte) (int, error) {
	return 0, errorRead
}

func TestFile_createJSONReader_ValidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	// Expectations
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
	).AnyTimes()

	f := &File{
		name:    "data.json",
		body:    io.NopCloser(strings.NewReader(`[{"id":1}]`)),
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	reader, err := f.createJSONReader("test-location")

	require.NoError(t, err)
	require.NotNil(t, reader)
	_, ok := reader.(*jsonReader)
	require.True(t, ok, "Expected *jsonReader for valid JSON")
}

func TestFile_createJSONReader_ValidJSONObject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
	).AnyTimes()

	f := &File{
		name:    "config.json",
		body:    io.NopCloser(strings.NewReader(`{"name":"test"}`)),
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	reader, err := f.createJSONReader("test-location")

	require.NoError(t, err)
	require.NotNil(t, reader)
	_, ok := reader.(*jsonReader)
	require.True(t, ok, "Expected *jsonReader for JSON object")
}

func TestFile_createJSONReader_ReadFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Errorf(
		"failed to read JSON body from location %s: %v",
		"test-location",
		errorRead,
	).Times(1)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
	).AnyTimes()

	f := &File{
		name:    "broken.json",
		body:    io.NopCloser(failingReader{}),
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	reader, err := f.createJSONReader("test-location")

	require.Error(t, err)
	require.Nil(t, reader)
}

func TestFile_createTextCSVReader_ValidText(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
	).AnyTimes()

	f := &File{
		name:    "data.txt",
		body:    io.NopCloser(strings.NewReader("line1\nline2")),
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	reader, err := f.createTextCSVReader("test-location")

	require.NoError(t, err)
	require.NotNil(t, reader)
	_, ok := reader.(*textReader)
	require.True(t, ok, "Expected *textReader for valid text file")
}

func TestFile_createTextCSVReader_EmptyText(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
	).AnyTimes()

	f := &File{
		name:    "empty.txt",
		body:    io.NopCloser(strings.NewReader("")),
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	reader, err := f.createTextCSVReader("test-location")

	require.NoError(t, err)
	require.NotNil(t, reader)
	_, ok := reader.(*textReader)

	require.True(t, ok, "Expected *textReader even for empty content")
}

func TestFile_createTextCSVReader_ReadFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Errorf(
		"failed to read text/csv body from location %s: %v",
		"test-location",
		errorRead,
	).Times(1)

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
	).AnyTimes()

	f := &File{
		name:    "broken.txt",
		body:    io.NopCloser(failingReader{}),
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	reader, err := f.createTextCSVReader("test-location")

	require.Error(t, err)
	require.Nil(t, reader)
}

func TestJSONReader_Next_Scan(t *testing.T) {
	jsonData := `[{"name":"John","age":30},{"name":"Jane","age":25}]`
	decoder := json.NewDecoder(strings.NewReader(jsonData))

	token, err := decoder.Token()
	require.NoError(t, err)
	require.Equal(t, json.Delim('['), token)

	reader := &jsonReader{
		decoder: decoder,
		token:   token,
	}

	count := 0

	for reader.Next() {
		var person struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		err := reader.Scan(&person)
		require.NoError(t, err)
		require.NotEmpty(t, person.Name)

		count++
	}

	require.Equal(t, 2, count)
}

func TestTextReader_Next_Scan(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)

	textData := "line1\nline2\nline3"
	reader := &textReader{
		scanner: bufio.NewScanner(strings.NewReader(textData)),
		logger:  mockLogger,
	}

	lines := []string{}

	for reader.Next() {
		var line string

		err := reader.Scan(&line)
		require.NoError(t, err)

		lines = append(lines, line)
	}

	require.Equal(t, []string{"line1", "line2", "line3"}, lines)
}

func TestTextReader_Scan_NonPointer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)

	reader := &textReader{
		scanner: bufio.NewScanner(strings.NewReader("test")),
		logger:  mockLogger,
	}

	reader.Next()

	var nonPointer string

	err := reader.Scan(nonPointer)
	require.Error(t, err)
	require.Equal(t, errStringNotPointer, err)
}

func TestFile_ModTime(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()

	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
	).AnyTimes()

	expectedTime := time.Now()
	f := &File{
		name:         "test.txt",
		lastModified: expectedTime,
		logger:       mockLogger,
		metrics:      mockMetrics,
	}

	result := f.ModTime()
	require.Equal(t, expectedTime, result)
}

func TestFile_Mode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()

	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
	).AnyTimes()

	tests := []struct {
		name     string
		isDir    bool
		expected fs.FileMode
	}{
		{name: "directory", isDir: true, expected: fs.ModeDir},
		{name: "file", isDir: false, expected: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{
				name:    "test",
				isDir:   tt.isDir,
				logger:  mockLogger,
				metrics: mockMetrics,
			}

			result := f.Mode()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFile_Sys(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()

	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), appFTPStats, gomock.Any(),
		"type", gomock.Any(),
		"status", gomock.Any(),
	).AnyTimes()

	f := &File{
		name:    "test.txt",
		logger:  mockLogger,
		metrics: mockMetrics,
	}

	result := f.Sys()
	require.Nil(t, result)
}
