// Package responder provides the functionality to populate the responses for the
// respective incoming requests
package responder

import (
	"fmt"

	"gofr.dev/pkg/gofr/template"
	"gofr.dev/pkg/log"
)

// CMD (Command) represents a command-line application configuration, including the ability to log messages.
//
// The `CMD` type encapsulates configuration settings and tools for command-line applications,
// including a logger object (`Logger`) that can be used to log messages and output information.
//
// It serves as a structured configuration for building command-line applications with logging capabilities.
type CMD struct {
	// Logger is a logger object that can be used to log messages.
	Logger log.Logger
}

// Respond logs errors and prints responses based on the data and error provided.
// If an error is not nil, it logs the error using the provided logger and also prints it to the standard output.
// If the data is of type template.Template, it renders and prints the template content.
// If the data is of type template.File, it prints the content of the file.
// For other data types, it prints the data to the standard output.
func (c *CMD) Respond(data interface{}, err error) {
	// added the logger to log the error in case of CMD application
	if err != nil {
		// since it a cmd err, we are logging as well as giving the response using fmt.Println.
		c.Logger.Error(err)
		fmt.Println(err)

		return
	}

	if d, ok := data.(template.Template); ok {
		var b []byte
		b, err = d.Render()

		if err != nil {
			c.Logger.Error(err)
			fmt.Println(err)

			return
		}

		fmt.Println(string(b))

		return
	}

	if f, ok := data.(template.File); ok {
		fmt.Println(string(f.Content))

		return
	}

	fmt.Println(data)
}
