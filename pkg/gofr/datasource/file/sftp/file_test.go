package sftp

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// newInMemClient returns an SFTP client talking to an in-process, in-memory SFTP server over net.Pipe.
func newInMemClient(t *testing.T) *sftp.Client {
	t.Helper()

	serverConn, clientConn := net.Pipe()

	server := sftp.NewRequestServer(serverConn, sftp.InMemHandler())

	go func() { _ = server.Serve() }()

	client, err := sftp.NewClientPipe(clientConn, clientConn)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	return client
}

// writeRemoteFile creates name on the in-memory server with content and returns it re-opened for reading.
func writeRemoteFile(t *testing.T, client *sftp.Client, name, content string) *sftp.File {
	t.Helper()

	require.NoError(t, client.MkdirAll(filepath.Dir(name)))

	f, err := client.Create(name)
	require.NoError(t, err)

	_, err = f.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	opened, err := client.Open(name)
	require.NoError(t, err)

	return opened
}

func TestSftpFile_FileInfo(t *testing.T) {
	client := newInMemClient(t)

	openFile := writeRemoteFile(t, client, "/info/data.txt", "hello")

	stat, err := openFile.Stat()
	require.NoError(t, err)

	tests := []struct {
		desc       string
		file       *sftp.File
		mockCall   func(l *MockLogger)
		expSize    int64
		expModTime time.Time
		expIsDir   bool
		expMode    os.FileMode
		expSys     any
	}{
		{
			desc: "stat fails on closed file",
			file: &sftp.File{},
			mockCall: func(l *MockLogger) {
				l.EXPECT().Errorf("failed to get file size: %v", os.ErrClosed)
				l.EXPECT().Errorf("failed to get file modification time: %v", os.ErrClosed)
				l.EXPECT().Errorf("failed to check if file is directory: %v", os.ErrClosed)
				l.EXPECT().Errorf("failed to get file mode: %v", os.ErrClosed)
				l.EXPECT().Errorf("failed to get file system info: %v", os.ErrClosed)
			},
			expSize:    0,
			expModTime: time.Unix(0, 0),
			expIsDir:   false,
			expMode:    0,
			expSys:     nil,
		},
		{
			desc:       "stat succeeds on open file",
			file:       openFile,
			mockCall:   func(*MockLogger) {},
			expSize:    5,
			expModTime: stat.ModTime(),
			expIsDir:   false,
			expMode:    stat.Mode(),
			expSys:     stat.Sys(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			logger := NewMockLogger(gomock.NewController(t))
			tc.mockCall(logger)

			f := sftpFile{File: tc.file, logger: logger}

			assert.Equal(t, tc.expSize, f.Size())
			assert.Equal(t, tc.expModTime, f.ModTime())
			assert.Equal(t, tc.expIsDir, f.IsDir())
			assert.Equal(t, tc.expMode, f.Mode())
			assert.Equal(t, tc.expSys, f.Sys())
		})
	}
}

func TestSftpFile_ReadAll_Text(t *testing.T) {
	client := newInMemClient(t)

	f := sftpFile{File: writeRemoteFile(t, client, "/text/data.csv", "id,name\n1,gofr"), logger: nil}

	reader, err := f.ReadAll()
	require.NoError(t, err)

	lines := make([]string, 0)

	for reader.Next() {
		var line string

		require.NoError(t, reader.Scan(&line))

		lines = append(lines, line)
	}

	assert.Equal(t, []string{"id,name", "1,gofr"}, lines)
}

func TestTextReader_Scan(t *testing.T) {
	client := newInMemClient(t)

	line := "line"

	tests := []struct {
		desc      string
		target    any
		expTarget any
		expErr    error
	}{
		{desc: "pointer to string", target: new(string), expTarget: &line, expErr: nil},
		{desc: "not a pointer to string", target: new(int), expTarget: new(int), expErr: errNotStringPointer},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			f := sftpFile{File: writeRemoteFile(t, client, "/scan/data.txt", "line")}

			reader, err := f.ReadAll()
			require.NoError(t, err)
			require.True(t, reader.Next())

			err = reader.Scan(tc.target)

			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expTarget, tc.target)
		})
	}
}

func TestSftpFile_ReadAll_JSONArray(t *testing.T) {
	client := newInMemClient(t)

	f := sftpFile{File: writeRemoteFile(t, client, "/json/array.json", `[{"name":"gofr"}]`)}

	reader, err := f.ReadAll()
	require.NoError(t, err)

	jr, ok := reader.(*jsonReader)
	require.True(t, ok)
	assert.Equal(t, json.Delim('['), jr.token)
}

func TestJSONReader_Scan(t *testing.T) {
	client := newInMemClient(t)

	type row struct {
		Name string `json:"name"`
	}

	tests := []struct {
		desc    string
		content string
		expRows []row
		expErr  bool
	}{
		{desc: "every element in order", content: `[{"name":"a"},{"name":"b"}]`, expRows: []row{{"a"}, {"b"}}},
		{desc: "empty array", content: `[]`, expRows: []row{}},
		{desc: "single object", content: `{"name":"a"}`, expRows: []row{{"a"}}},
		{desc: "element of the wrong type", content: `["a"]`, expRows: []row{}, expErr: true},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// A strict mock: reading a valid file must log nothing.
			f := sftpFile{
				File:   writeRemoteFile(t, client, fmt.Sprintf("/json/scan-%d.json", i), tc.content),
				logger: NewMockLogger(gomock.NewController(t)),
			}

			reader, err := f.ReadAll()
			require.NoError(t, err)

			rows := make([]row, 0)

			for reader.Next() {
				var r row

				if err = reader.Scan(&r); err != nil {
					break
				}

				rows = append(rows, r)
			}

			if tc.expErr {
				var typeErr *json.UnmarshalTypeError
				require.ErrorAs(t, err, &typeErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.expRows, rows)
		})
	}
}

func TestJSONReader_RewindError(t *testing.T) {
	client := newInMemClient(t)
	logger := NewMockLogger(gomock.NewController(t))
	f := sftpFile{File: writeRemoteFile(t, client, "/json/rewind.json", `{"name":"a"}`), logger: logger}

	// A closed file cannot be rewound, which ReadAll cannot reach: its first token read fails first.
	require.NoError(t, f.File.Close())
	logger.EXPECT().Errorf("failed to rewind JSON file for reading as object %v", gomock.Any())

	reader, err := f.createJSONObjectReader()

	require.ErrorIs(t, err, os.ErrClosed)
	assert.Nil(t, reader)
}

func TestSftpFile_ReadAll_JSONErrors(t *testing.T) {
	client := newInMemClient(t)

	tests := []struct {
		desc     string
		name     string
		content  string
		mockCall func(l *MockLogger)
		expErr   error
	}{
		{
			desc:    "empty json file",
			name:    "/errors/empty.json",
			content: "",
			mockCall: func(l *MockLogger) {
				l.EXPECT().Errorf("failed to decode JSON token %v", io.EOF)
			},
			expErr: io.EOF,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			logger := NewMockLogger(gomock.NewController(t))
			tc.mockCall(logger)

			f := sftpFile{File: writeRemoteFile(t, client, tc.name, tc.content), logger: logger}

			reader, err := f.ReadAll()

			require.ErrorIs(t, err, tc.expErr)
			assert.Nil(t, reader)
		})
	}
}
