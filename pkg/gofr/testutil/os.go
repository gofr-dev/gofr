package testutil

import (
	"io/ioutil"
	"os"
)

func StdoutOutputForFunc(f func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	f()

	w.Close()

	out, _ := ioutil.ReadAll(r)
	os.Stdout = old

	return string(out)
}

func StderrOutputForFunc(f func()) string {
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w

	f()

	w.Close()

	out, _ := ioutil.ReadAll(r)
	os.Stderr = old

	return string(out)
}
