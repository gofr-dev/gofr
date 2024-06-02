package gofr

import (
	"os"
	"regexp"
	"strings"

	"gofr.dev/pkg/gofr/container"

	cmd2 "gofr.dev/pkg/gofr/cmd"
)

type cmd struct {
	routes      []route
	description string
}

type route struct {
	pattern     string
	handler     Handler
	help        string
	fullPattern string
}

type Options func(c *route)

type ErrCommandNotFound struct{}

func (e ErrCommandNotFound) Error() string {
	return "No Command Found!" //nolint:goconst // This error is needed and repetition is in test to check for the exact string.
}

func (cmd *cmd) Validate(data []string) bool {
	for _, val := range data {
		if val != "" {
			return false
		}
	}

	return true
}

func (cmd *cmd) Run(c *container.Container) {
	args := os.Args[1:] // First one is command itself
	command := []string{}
	tempCommand := ""

	// Removing all flags and putting everything else as a part of command.
	// So, unlike native flag package we can put subcommands anywhere
	for _, a := range args {
		if a == "" {
			continue // This takes cares of cases where command has multiple space in between.
		}

		if a[0] != '-' {
			tempCommand = tempCommand + " " + a
		} else {
			command = append(command, tempCommand)
			tempCommand = a
		}
	}

	if tempCommand != "" {
		command = append(command, tempCommand)
	}

	ctx := newContext(&cmd2.Responder{}, cmd2.NewRequest(command), c)

	for it, commandVal := range command {
		if commandVal == "" {
			continue
		}

		h := cmd.handler(commandVal)

		if h == nil {
			ctx.responder.Respond(nil, ErrCommandNotFound{})
			return
		}

		ctx.responder.Respond(h(ctx))

		if it != len(command) {
			ctx.responder.Respond("\n", nil)
		}
	}
}

func (cmd *cmd) handler(path string) Handler {
	if len(path) > 1 && path[:2] == "--" {
		path = path[2:]
	} else if path[0] == '-' {
		path = path[1:]
	}

	if strings.Contains(path, " ") {
		path = strings.Split(path, " ")[0]
	}

	for _, route := range cmd.routes {
		re := regexp.MustCompile(route.pattern)

		if cmd.Validate(re.Split(path, -1)) {
			return route.handler
		}

		if route.fullPattern != "nil" {

			reFullPattern := regexp.MustCompile(route.fullPattern)

			if cmd.Validate(reFullPattern.Split(path, -1)) {
				return route.handler
			}
		}
	}

	return nil
}

func AddHelp(helperString string) Options {
	return func(r *route) {
		r.help = helperString
	}
}

func AddFullPattern(fullPattern string) Options {
	return func(r *route) {
		r.fullPattern = fullPattern
	}
}

func (cmd *cmd) addRoute(pattern string, handler Handler, options ...Options) {
	tempRoute := route{
		pattern:     pattern,
		handler:     handler,
		help:        "help message not provided",
		fullPattern: "nil",
	}

	for _, opt := range options {
		opt(&tempRoute)
	}

	cmd.routes = append(cmd.routes, tempRoute)
}
