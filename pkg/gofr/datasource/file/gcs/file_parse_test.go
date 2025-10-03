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

func TestFile_ReadAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name       string
		fileName   string
		body       io.ReadCloser
		expectJSON bool
		expectErr  bool
	}{
		{
			name:       "JSON file",
			fileName:   "test.json",
			body:       io.NopCloser(strings.NewReader(`[{"key":"value"}]`)),
			expectJSON: true,
			expectErr:  false,
		},
		{
			name:       "Text file",
			fileName:   "test.txt",
			body:       io.NopCloser(strings.NewReader("line1\nline2")),
			expectJSON: false,
			expectErr:  false,
		},
		{
			name:       "CSV file",
			fileName:   "test.csv",
			body:       io.NopCloser(strings.NewReader("col1,col2\nval1,val2")),
			expectJSON: false,
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := NewMockLogger(ctrl)
			mockMetrics := NewMockMetrics(ctrl)

			mockLogger.EXPECT().Debugf(gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), appFTPStats, gomock.Any(),
				"type", gomock.Any(), "status", gomock.Any()).AnyTimes()

			f := &File{
				name:    tt.fileName,
				body:    tt.body,
				logger:  mockLogger,
				metrics: mockMetrics,
			}

			reader, err := f.ReadAll()

			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, reader)

			if tt.expectJSON {
				_, ok := reader.(*jsonReader)
				require.True(t, ok, "Expected jsonReader")
			} else {
				_, ok := reader.(*textReader)
				require.True(t, ok, "Expected textReader")
			}
		})
	}
}

type failingReader struct{}

func (failingReader) Read(_ []byte) (int, error) {
	return 0, errorRead
}

func TestFile_createJSONReader(t *testing.T) {
	tests := []struct {
		name      string
		body      io.ReadCloser
		expectErr bool
	}{
		{
			name:      "valid JSON array",
			body:      io.NopCloser(strings.NewReader(`[{"key":"value"}]`)),
			expectErr: false,
		},
		{
			name:      "valid JSON object",
			body:      io.NopCloser(strings.NewReader(`{"key":"value"}`)),
			expectErr: false,
		},
		{
			name:      "read body failure",
			body:      io.NopCloser(failingReader{}),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				name:    "test.json",
				body:    tt.body,
				logger:  mockLogger,
				metrics: mockMetrics,
			}

			reader, err := f.createJSONReader("test-location")

			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, reader)
			_, ok := reader.(*jsonReader)
			require.True(t, ok)
		})
	}
}

func TestFile_createTextCSVReader(t *testing.T) {
	tests := []struct {
		name      string
		body      io.ReadCloser
		expectErr bool
	}{
		{
			name:      "valid text",
			body:      io.NopCloser(strings.NewReader("line1\nline2")),
			expectErr: false,
		},
		{
			name:      "empty text",
			body:      io.NopCloser(strings.NewReader("")),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				body:    tt.body,
				logger:  mockLogger,
				metrics: mockMetrics,
			}

			reader, err := f.createTextCSVReader("test-location")

			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, reader)
			_, ok := reader.(*textReader)
			require.True(t, ok)
		})
	}
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
