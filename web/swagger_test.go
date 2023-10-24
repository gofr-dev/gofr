package web

import (
	"errors"
	fs2 "io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSwaggerFile(t *testing.T) {
	testcases := []struct {
		filename string
		hasData  bool // To check if the file is not empty. All the below files should not be empty in order for swagger to work
		expErr   error
	}{
		{"index.html", true, nil},
		{"swagger-ui.css", true, nil},
		{"favicon-32x32.png", true, nil},
		{"favicon-16x16.png", true, nil},
		{"swagger-ui-bundle.js", true, nil},
		{"swagger-ui-standalone-preset.js", true, nil},
		{"some-random-file.txt", false, &fs2.PathError{
			Op:   "open",
			Path: "swagger/some-random-file.txt",
			Err:  errors.New("file does not exist"),
		}},
	}

	for _, tc := range testcases {
		data, _, err := GetSwaggerFile(tc.filename)
		assert.Equal(t, tc.expErr, err)

		if tc.hasData && len(data) == 0 {
			t.Errorf("File %v is empty! It should contain data!", tc.filename)
		}
	}
}
