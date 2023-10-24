package file

import (
	"testing"
)

func TestListingNotSupported_Error(t *testing.T) {
	err := ErrListingNotSupported
	expected := "Listing not supported for provided file store. Please set a valid value of FILE_STORE:{LOCAL or SFTP}"

	if err.Error() != expected {
		t.Errorf("FAILED, Expected: %v, Got: %v", expected, err)
	}
}
