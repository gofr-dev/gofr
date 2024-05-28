package gofr

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gofr.dev/pkg/gofr/container"

	cmd2 "gofr.dev/pkg/gofr/cmd"
)

type cmd struct {
	routes []route
}

type route struct {
	pattern     string
	handler     Handler
	description string
}

type ErrCommandNotFound struct{}

func (e ErrCommandNotFound) Error() string {
	return "No Command Found!" //nolint:goconst // This error is needed and repetition is in test to check for the exact string.
}

func (cmd *cmd) Run(c *container.Container) {
	args := os.Args[1:] // First one is command itself
	command := ""

	showHelp := false
	for _, a := range args {
		if a == "" {
			continue // This takes care of cases where command has multiple spaces in between.
		}

		if a == "-h" || a == "--help" {
			showHelp = true
			continue
		}

		if a[0] != '-' {
			command = command + " " + a
		}
	}

	if showHelp || command == "" {
		cmd.printHelp()
		return
	}

	h := cmd.handler(strings.TrimSpace(command))
	ctx := newContext(&cmd2.Responder{}, cmd2.NewRequest(args), c)

	if h == nil {
		fmt.Println("Unknown command:", command)
		cmd.printHelp()
		ctx.responder.Respond(nil, ErrCommandNotFound{})
		return
	}

	ctx.responder.Respond(h(ctx))
}

func (cmd *cmd) handler(path string) Handler {
	for _, route := range cmd.routes {
		re := regexp.MustCompile(route.pattern)
		if re.MatchString(path) {
			return route.handler
		}
	}

	return nil
}

func (cmd *cmd) addRoute(pattern string, handler Handler, description string) {
	cmd.routes = append(cmd.routes, route{
		pattern:     pattern,
		handler:     handler,
		description: description,
	})
}

func (cmd *cmd) printHelp() {
	fmt.Println("Available commands:")
	for _, route := range cmd.routes {
		fmt.Printf("  %s: %s\n", route.pattern, route.description)
	}
}
