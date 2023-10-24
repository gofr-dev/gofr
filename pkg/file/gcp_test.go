package file

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	gofrErr "gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/config"
)

func TestNewGCP(t *testing.T) {
	c := &config.MockConfig{Data: map[string]string{
		"FILE_STORE":              "GCP",
		"GCP_STORAGE_CREDENTIALS": "gcpKey",
		"GCP_STORAGE_BUCKET_NAME": "gcpBucket",
	}}

	_, err := NewWithConfig(c, "test.txt", READ)
	if err == nil {
		t.Errorf("For wrong config GCP client should not be created")
	}
}
func Test_list_gcp(t *testing.T) {
	s := &gcp{}
	expErr := ErrListingNotSupported
	_, err := s.list("test")
	assert.Equalf(t, expErr, err, "Test case failed.\nExpected: %v, got: %v", expErr, err)
}

func Test_gcp_fetch(t *testing.T) {
	gcpFile := &gcp{}
	err := gofrErr.Error("storage: bucket name is empty")

	localFile := newLocalFile("", READWRITE)
	_ = localFile.Open()

	resp := gcpFile.fetch(localFile.FD)

	if reflect.DeepEqual(resp, err) {
		t.Errorf("expected: %v, got: %v", resp, err)
	}

	_ = localFile.Close()
}

func Test_gcp_push(t *testing.T) {
	gcpFile := &gcp{}

	localFile := newLocalFile("", READWRITE)

	_ = localFile.Open()

	resp := gcpFile.push(localFile.FD)

	if reflect.DeepEqual(resp, nil) {
		t.Errorf("expected: %v, got: %v", resp, nil)
	}

	_ = localFile.Close()
}

func Test_move_gcp(t *testing.T) {
	source := "source/path/file.txt"
	destination := "destination/path/file.txt"
	gcpInstance := &gcp{}
	err := gcpInstance.move(source, destination)

	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}
