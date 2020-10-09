package cmd

import (
	"fmt"
	"os"
)

type Responder struct{}

func (r *Responder) Respond(data interface{}, err error) {
	if data != nil {
		fmt.Println(data)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	os.Exit(0)
}
