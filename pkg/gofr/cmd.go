package gofr

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	cmd2 "gofr.dev/pkg/gofr/cmd"
	"gofr.dev/pkg/gofr/container"
)

type cmd struct {
	routes []route
}

type route struct {
	pattern     string
	handler     Handler
	description string
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
			tempCommand = tempCommand + " " + a
		} else {
			command = append(command, tempCommand)
			tempCommand = a
		}
	}

	if tempCommand != "" {
		command = append(command, tempCommand)
	}

	if showHelp || len(command) == 0 {
		cmd.printHelp()
		return
	}

	ctx := newContext(&cmd2.Responder{}, cmd2.NewRequest(command), c)

	for it, commandVal := range command {
		if commandVal == "" {
			continue
		}

		h := cmd.handler(commandVal)

		if h == nil {
			cmd.printHelp()
			ctx.responder.Respond(nil, ErrCommandNotFound{})
			return
		}

		ctx.responder.Respond(h(ctx))

		if it != len(command)-1 {
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

func AddDescription(descString string) Options {
	return func(r *route) {
		r.description = descString
	}
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
		description: "description message not provided",
		help:        "help message not provided",
		fullPattern: "nil",
	}

	for _, opt := range options {
		opt(&tempRoute)
	}

	cmd.routes = append(cmd.routes, tempRoute)
}

func (cmd *cmd) printHelp() {
	fmt.Println("Available commands:")
	for _, route := range cmd.routes {
		fmt.Printf("\n  %s\n", route.pattern)
		if route.description != "" {
			fmt.Printf("    Description: %s\n", route.description)
		}
		if route.help != "" {
			fmt.Printf("    Help: %s\n", route.help)
		}
	}
}
