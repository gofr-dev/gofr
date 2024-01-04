package testutil

import (
	"io"
	"os"
)

func StdoutOutputForFunc(f func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	f()

	_ = w.Close()

	out, _ := io.ReadAll(r)
	os.Stdout = old

	return string(out)
}

func StderrOutputForFunc(f func()) string {
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w

	f()

	_ = w.Close()

	out, _ := io.ReadAll(r)
	os.Stderr = old

	return string(out)
}
