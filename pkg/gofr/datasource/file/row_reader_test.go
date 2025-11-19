package file

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type errReader struct {
	err error
}

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func TestNewJSONReader_ReadErrorReturnsError(t *testing.T) {
	readErr := io.ErrUnexpectedEOF
	_, err := NewJSONReader(errReader{err: readErr}, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, readErr)
}

func TestNewJSONReader_EmptyInputReturnsEOF(t *testing.T) {
	_, err := NewJSONReader(strings.NewReader("   \n\t  "), nil)
	require.Error(t, err)
	assert.Equal(t, io.EOF, err)
}

func TestNewJSONReader_InvalidJSONReturnsError(t *testing.T) {
	_, err := NewJSONReader(strings.NewReader("{"), nil)
	require.Error(t, err)
	// errInvalidJSON is defined in row_reader.go
	assert.ErrorIs(t, err, errInvalidJSON)
}

func TestJSONReader_ArrayAndSingleObject_Behavior(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	arrayJSON := `[{"a":1},{"a":2}]`
	jr, err := NewJSONReader(strings.NewReader(arrayJSON), NewMockLogger(ctrl))
	require.NoError(t, err)

	require.True(t, jr.Next())

	var obj map[string]any

	require.NoError(t, jr.Scan(&obj))
	assert.InEpsilon(t, float64(1), obj["a"].(float64), 1e-9)

	require.True(t, jr.Next())

	var obj2 map[string]any

	require.NoError(t, jr.Scan(&obj2))
	assert.InEpsilon(t, float64(2), obj2["a"].(float64), 1e-9)

	assert.False(t, jr.Next())

	// Single object: Next true once, Scan once, then subsequent Scan returns io.EOF
	single := `{"b":3}`

	jr2, err := NewJSONReader(strings.NewReader(single), nil)
	require.NoError(t, err)

	require.True(t, jr2.Next())

	var so map[string]any

	require.NoError(t, jr2.Scan(&so))
	assert.InEpsilon(t, float64(3), so["b"].(float64), 1e-9)

	errEOF := jr2.Scan(&so)

	assert.Equal(t, io.EOF, errEOF)
}

func TestNewJSONReader_ErrorOnInvalidJSON(t *testing.T) {
	invalid := `{`

	_, err := NewJSONReader(strings.NewReader(invalid), nil)

	require.Error(t, err)
}

func TestTextReader_NextAndScan_SuccessAndTypeError(t *testing.T) {
	r := strings.NewReader("line1\nline2\n")
	tr := NewTextReader(r, nil)

	require.True(t, tr.Next())

	var s string

	require.NoError(t, tr.Scan(&s))
	assert.Equal(t, "line1", s)

	require.True(t, tr.Next())
	require.NoError(t, tr.Scan(&s))

	assert.Equal(t, "line2", s)

	assert.False(t, tr.Next())

	// Scan with wrong type should return errNotPointer
	err := tr.Scan(new(int))
	assert.Equal(t, errNotPointer, err)
}

// Ensure ListDir returns object infos (else branch) and directory names.
func TestLocalProvider_ListDir_ReturnsObjectsAndDirs(t *testing.T) {
	provider := &localProvider{}
	tmp := t.TempDir()

	// create a top-level file and a sub-directory with a nested file
	err := os.MkdirAll(filepath.Join(tmp, "subdir"), 0o755)
	require.NoError(t, err)

	filePath := filepath.Join(tmp, "file.txt")
	err = os.WriteFile(filePath, []byte("hello"), 0o600)
	require.NoError(t, err)

	nestedFile := filepath.Join(tmp, "subdir", "nested.txt")
	err = os.WriteFile(nestedFile, []byte("x"), 0o600)
	require.NoError(t, err)

	objects, dirs, err := provider.ListDir(t.Context(), tmp)
	require.NoError(t, err)

	// verify directory list contains trailing slash
	assert.Contains(t, dirs, "subdir/")

	// verify objects contains the top-level file info
	foundFile := false

	for _, oi := range objects {
		if oi.Name == "file.txt" && !oi.IsDir {
			foundFile = true

			break
		}
	}

	assert.True(t, foundFile, "expected to find top-level file in objects")
}
