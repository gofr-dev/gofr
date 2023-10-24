package errors

import (
	"testing"
)

func TestInvalidFileStorage_Error(t *testing.T) {
	err := InvalidFileStorage
	expected := "Invalid File Storage.Please set a valid value of FILE_STORE:{LOCAL or AZURE or GCP or AWS}"

	if err.Error() != expected {
		t.Errorf("FAILED, Expected: %v, Got: %v", expected, err)
	}
}
