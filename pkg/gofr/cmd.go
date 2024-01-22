package gofr

import (
	"os"
	"regexp"

	"gofr.dev/pkg/gofr/container"

	cmd2 "gofr.dev/pkg/gofr/cmd"
)

type cmd struct {
	routes []route
}

type route struct {
	pattern string
	handler Handler
}

type ErrCommandNotFound struct{}

func (e ErrCommandNotFound) Error() string {
	return "No Command Found!" //nolint:goconst // This error is needed and repetition is in test to check for the exact string.
}

func (cmd *cmd) Run(c *container.Container) {
	args := os.Args[1:] // First one is command itself
	command := ""

	// Removing all flags and putting everything else as a part of command.
	// So, unlike native flag package we can put subcommands anywhere
	for _, a := range args {
		if a == "" {
			continue // This takes cares of cases where command has multiple space in between.
		}

		if a[0] != '-' {
			command = command + " " + a
		}
	}

	h := cmd.handler(command)
	ctx := newContext(&cmd2.Responder{}, cmd2.NewRequest(args), c)

	if h == nil {
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

func (cmd *cmd) addRoute(pattern string, handler Handler) {
	cmd.routes = append(cmd.routes, route{
		pattern: pattern,
		handler: handler,
	})
}
