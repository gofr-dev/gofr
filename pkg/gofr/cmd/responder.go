package cmd

import (
	"fmt"
	"os"
)

type Responder struct{}

func (*Responder) Respond(data any, err error) {
	// TODO - provide proper exit codes here. Using os.Exit directly is a problem for tests.
	if data != nil {
		fmt.Fprintln(os.Stdout, data)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
