package file

import (
	"io"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
)

const testFile = "/tmp/testData.txt"

type mockRemoteFileAbstractor struct {
	fetchFunc func(*os.File) error
}

func (m *mockRemoteFileAbstractor) SetFetchFunc(f func(*os.File) error) {
	m.fetchFunc = f
}

func (m *mockRemoteFileAbstractor) fetch(fd *os.File) error {
	if m.fetchFunc != nil {
		return m.fetchFunc(fd)
	}

	return errors.New("unimplemented: fetch")
}

func (m *mockRemoteFileAbstractor) push(_ *os.File) error {
	return errors.New("unimplemented: push") // stub behavior
}

func (m *mockRemoteFileAbstractor) list(_ string) ([]string, error) {
	return nil, errors.New("unimplemented: list") // stub behavior
}

func (m *mockRemoteFileAbstractor) move(_, _ string) error {
	return errors.New("unimplemented: move") // stub behavior
}

func TestLocalFileOpen(t *testing.T) {
	testcases := []struct {
		filename string
		mode     Mode
		expErr   error
	}{
		{"newTest.txt", READ, &os.PathError{
			Op:   "open",
			Path: "/tmp/newTest.txt",
			Err:  syscall.ENOENT,
		}}, // opening a new file in read mode does not make sense!
		{"test.txt", WRITE, nil},
		{"test.txt", READ, nil},
		{"test1.txt", READWRITE, nil},
		{"test1.txt", APPEND, nil},
		{"test1.txt", "unknown", nil},
	}

	c := &config.MockConfig{Data: map[string]string{
		"FILE_STORE": "LOCAL",
	}}

	for _, tc := range testcases {
		f, err := NewWithConfig(c, "/tmp/"+tc.filename, tc.mode)
		if err != nil {
			t.Error(err)
		}

		err = f.Open()
		assert.Equal(t, tc.expErr, err)
	}
}

func TestOpen_Combined(t *testing.T) {
	openTestCases := []struct {
		desc          string
		fileName      string
		fileMode      int
		mockFetchFunc func(*os.File) error
		expectedError bool
		expectedMode  int
	}{
		{
			desc:          "Success: Download and open in READ mode",
			fileName:      "testfile.txt",
			fileMode:      fetchLocalFileMode(READ),
			mockFetchFunc: func(fd *os.File) error { return nil },
			expectedError: false,
			expectedMode:  fetchLocalFileMode(READWRITE),
		},
		{
			desc:          "Error: Download fails",
			fileName:      "another.doc",
			fileMode:      fetchLocalFileMode(READ),
			mockFetchFunc: func(fd *os.File) error { return errors.New("mocked error") },
			expectedError: true,
			expectedMode:  fetchLocalFileMode(READWRITE),
		},
		{
			desc:          "Error: Unexpected EOF during download but fileMode is Append",
			fileName:      "bigfile.zip",
			fileMode:      fetchLocalFileMode(APPEND),
			mockFetchFunc: func(fd *os.File) error { return io.EOF },
			expectedError: false,
			expectedMode:  522,
		},
	}

	for _, tc := range openTestCases {
		l := &fileAbstractor{
			fileName:             tc.fileName,
			fileMode:             tc.fileMode,
			remoteFileAbstracter: &mockRemoteFileAbstractor{},
		}

		mockAbstractor := l.remoteFileAbstracter.(*mockRemoteFileAbstractor)
		mockAbstractor.SetFetchFunc(tc.mockFetchFunc)

		err := l.Open()
		if err != nil && !tc.expectedError {
			t.Errorf("Unexpected error: %v", err)
		} else if err == nil && tc.expectedError {
			t.Errorf("Expected error, got nil")
		}

		if !tc.expectedError && l.fileMode != tc.expectedMode {
			t.Errorf("Expected file mode %v after download, got %v", tc.expectedMode, l.fileMode)
		}
	}
}

func TestLocal_WriteInReadMode(t *testing.T) {
	c := &config.MockConfig{Data: map[string]string{
		"FILE_STORE": "LOCAL",
	}}

	err := createTestFile()
	if err != nil {
		t.Error(err)
	}

	f, err := NewWithConfig(c, testFile, READ)
	if err != nil {
		t.Error(err)
	}

	defer f.Close()

	err = f.Open()
	if err != nil {
		t.Error(err)
	}

	dataToWrite := []byte("The quick brown fox jumps over the lazy dog")

	_, err = f.Write(dataToWrite)
	if err == nil {
		t.Error("Expected error while writing to a Read only file!")
	}
}

func TestLocal_ReadInWriteMode(t *testing.T) {
	c := &config.MockConfig{Data: map[string]string{
		"FILE_STORE": "LOCAL",
	}}

	err := createTestFile()
	if err != nil {
		t.Error(err)
	}

	f, err := NewWithConfig(c, testFile, WRITE)
	if err != nil {
		t.Error(err)
	}

	defer f.Close()

	err = f.Open()
	if err != nil {
		t.Error(err)
	}

	b := make([]byte, 50)
	if _, err = f.Read(b); err == nil {
		t.Error("Expected error while reading from a Write only file!")
	}
}

func TestNilFileDescriptor(t *testing.T) {
	file := &fileAbstractor{FD: nil}
	b := make([]byte, 50)
	offset := int64(2)
	whence := 0

	_, err := file.Read(b)
	if err == nil {
		t.Error("Expected error while Reading from nil file descriptor")
	}

	_, err = file.Write(b)
	if err == nil {
		t.Error("Expected error while Writing from nil file descriptor")
	}

	err = file.Close()
	if err == nil {
		t.Error("Expected error while Closing nil file descriptor")
	}

	_, err = file.Seek(offset, whence)
	if err == nil {
		t.Error("Expected error while seeking nil file descriptor")
	}
}

func TestMove(t *testing.T) {
	testcases := []struct {
		desc           string
		fileAbstracter *fileAbstractor
		fileName       string
		expErr         error
	}{
		{"local move success", &fileAbstractor{}, testFile, nil},
	}

	err := createTestFile()
	if err != nil {
		t.Fatalf("Unable to create test file : %v", err)
	}

	// remove file which will be created on local move for case 1, testFile create will be moved to this new name
	defer os.Remove("testData1.txt")

	for i, tc := range testcases {
		err := tc.fileAbstracter.Move(tc.fileName, "testData1.txt")

		assert.Equalf(t, tc.expErr, err, "TestCase[%d] Failed. Expected %v Got %v", i, tc.expErr, err)
	}
}

func TestMoveFileDoesNotExist(t *testing.T) {
	file := &fileAbstractor{}

	err := file.Move("abc.txt", "xyz.txt")

	assert.NotNil(t, err, "TestCase Failed. Expected %v Got %v \n move failure when source doesn't exist", nil, err)
}

func TestCopy(t *testing.T) {
	file := &fileAbstractor{}

	count, err := file.Copy("files", "file/file.go")

	assert.Equal(t, 0, count, "Test case failed")

	assert.Nil(t, err, "Test case failed.")
}

func TestDelete(t *testing.T) {
	file := &fileAbstractor{FD: nil}

	err := file.Delete("abc.txt")

	assert.Nil(t, err, "Test case failed.")
}

func TestNotNilFileDescriptor(t *testing.T) {
	mode := fetchLocalFileMode(READWRITE)

	tests := []struct {
		fileMode         int
		str              string
		appendOrOverride string
		output           string
	}{
		{mode, "Test read write ", "Override the existing string", "Override the existing string"},
		{mode | os.O_APPEND, "Test read write ", "Append in the existing string", "Test read write Append in the existing string"},
	}

	for i, tc := range tests {
		fileName := "/tmp/testFile.txt"
		b := performFileOps(t, tc.fileMode, fileName, tc.str, tc.appendOrOverride)
		// if the file has been opened in READWRITE mode then tc.str content should get overwritten by tc.appendOrOverride
		// if it doesn't happen then through an error
		if tc.fileMode == mode {
			if strings.Contains(string(b), tc.str) {
				t.Errorf("Unexpected string: %v", tc.str)
			}
		}

		if !strings.Contains(string(b), tc.output) {
			t.Errorf("Failed[%v]Expect %v got %v", i, tc.output, string(b))
		}
	}
}

func performFileOps(t *testing.T, fileMode int, fileName, str, appendOrOverride string) []byte {
	b := make([]byte, 60)
	offset := int64(0)
	whence := 0
	l := fileAbstractor{fileName: fileName, fileMode: fileMode}

	if err := l.Open(); err != nil {
		t.Error(err)
	}

	defer os.Remove(fileName)

	if _, err := l.Write([]byte(str)); err != nil {
		t.Error(err)
	}
	// offset is set to the start of the file
	if _, err := l.Seek(offset, whence); err != nil {
		t.Error(err)
	}

	if _, err := l.Write([]byte(appendOrOverride)); err != nil {
		t.Error(err)
	}
	// offset is set to the start of the file
	if _, err := l.Seek(offset, whence); err != nil {
		t.Error(err)
	}

	if _, err := l.Read(b); err != nil {
		t.Error(err)
	}

	return b
}

func TestLocal_Seek(t *testing.T) {
	err := createTestFile()
	if err != nil {
		t.Error(err)
	}

	defer os.Remove(testFile)

	tests := []struct {
		mode   Mode
		offset int64
		whence int
	}{
		{READWRITE, 0, 0},
		{WRITE, 2, 0},
		{READ, 1, 2},
		{APPEND, 0, 0},
	}

	for i, tc := range tests {
		l := fileAbstractor{fileName: "testFile.txt", fileMode: fetchLocalFileMode(tc.mode)}
		if err := l.Open(); err != nil {
			t.Error(err)
		}

		offset, err := l.Seek(tc.offset, tc.whence)

		assert.Equal(t, tc.offset, offset, i)

		if err != nil {
			t.Errorf("expect nil got err %v", err)
		}

		if err := l.Close(); err != nil {
			t.Error(err)
		}
	}
}

func createTestFile() error {
	file, err := os.OpenFile(testFile, fetchLocalFileMode(WRITE), os.ModePerm)
	if err != nil {
		return err
	}

	defer func() {
		_ = file.Close()
	}()

	_, err = file.WriteString("The quick brown fox jumps over the lazy dog")

	return err
}

func Test_List(t *testing.T) {
	// Creating temporary directory for tests
	d := t.TempDir()
	_ = os.Chdir(d)

	// Creating two files in the temp directory
	_, _ = os.Create("test1.txt")
	_, _ = os.Create("test2.txt")

	expRes := []string{"test1.txt", "test2.txt"}

	// Initializing file abstracter
	l1 := newLocalFile("", "")
	l2 := newLocalFile("", "")
	l2.remoteFileAbstracter = &aws{}

	testcases := []struct {
		dir    string
		l      *fileAbstractor
		exp    []string
		expErr error
	}{
		{dir: d, l: l1, exp: expRes, expErr: nil},
		{dir: "abc", l: l1, exp: nil, expErr: &fs.PathError{Path: "abc"}},
		{dir: d, l: l2, exp: nil, expErr: ErrListingNotSupported},
	}

	for i, tc := range testcases {
		val, err := tc.l.List(tc.dir)
		assert.Equal(t, tc.exp, val, "Test failed %v. Expected %v, got %v", i, tc.exp, val)
		assert.IsType(t, tc.expErr, err, "Test failed %v. Expected: %v, got: %v", i, tc.expErr, err)
	}
}

// Test_createNestedDir to test behavior of createNestedDirSFTP method
func Test_createNestedDir(t *testing.T) {
	testPath := t.TempDir() + "path/to/test/directory"

	defer os.RemoveAll(testPath)

	err := createNestedDir(testPath)
	if err != nil {
		t.Fatalf("Error creating directory: %v", err)
	}

	// Check if the directory exists
	_, err = os.Stat(testPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Errorf("Test failed: Expected directory %s to exist, but it doesn't", testPath)
		} else {
			t.Errorf("Error checking directory existence: %v", err)
		}
	}
}
