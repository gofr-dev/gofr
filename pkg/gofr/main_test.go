package gofr

import (
	"io"
	"log"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)

	_ = New()

	os.Exit(m.Run())
}
