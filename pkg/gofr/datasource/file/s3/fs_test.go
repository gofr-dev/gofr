package s3

import (
	"testing"
)

func TestConnect(t *testing.T) {
	err := Connect()
	if err != nil {
		t.Error(err)
	}
}
