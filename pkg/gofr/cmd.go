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
}

type Options func(c *route)

type ErrCommandNotFound struct{}

func (e ErrCommandNotFound) Error() string {
	return "No Command Found!"
}

func (cmd *cmd) Run(c *container.Container) {
	args := os.Args[1:] // First one is command itself
	subCommand := ""
	showHelp := false

	for _, a := range args {
		if a == "" {
			continue // This takes care of cases where subCommand has multiple spaces in between.
		}

		if a == "-h" || a == "--help" {
			showHelp = true

			continue
		}

		if a[0] != '-' {
			subCommand = subCommand + " " + a
		}
	}

	if showHelp && subCommand == "" {
		cmd.printHelp()
		return
	}

	r := cmd.handler(subCommand)
	ctx := newContext(&cmd2.Responder{}, cmd2.NewRequest(args), c)

	// handling if route is not found or the handler is nil
	if cmd.noCommandResponse(r, ctx) {
		return
	}

	if showHelp {
		fmt.Println(r.help)
		return
	}

	ctx.responder.Respond(r.handler(ctx))
}

// noCommandResponse responds with error when no route with the given subcommand is not found or handler is nil.
func (cmd *cmd) noCommandResponse(r *route, ctx *Context) bool {
	if r == nil {
		ctx.responder.Respond(nil, ErrCommandNotFound{})
		cmd.printHelp()

		return true
	}

	if r.handler == nil {
		ctx.responder.Respond(nil, ErrCommandNotFound{})

		return true
	}

	return false
}

func (cmd *cmd) handler(path string) *route {
	// Trim leading dashes
	path = strings.TrimPrefix(strings.TrimPrefix(path, "--"), "-")

	// Iterate over the routes to find a matching handler
	for _, r := range cmd.routes {
		re := regexp.MustCompile(r.pattern)

		if re.MatchString(path) {
			return &r
		}
	}

	// Return nil if no handler matches
	return nil
}

// AddDescription adds the description text for a specified subcommand.
func AddDescription(descString string) Options {
	return func(r *route) {
		r.description = descString
	}
}

// AddHelp adds the helper text for the given subcommand
// this is displayed when -h or --help option/flag is provided.
func AddHelp(helperString string) Options {
	return func(r *route) {
		r.help = helperString
	}
}

func (cmd *cmd) addRoute(pattern string, handler Handler, options ...Options) {
	tempRoute := route{
		pattern: pattern,
		handler: handler,
	}

	for _, opt := range options {
		opt(&tempRoute)
	}

	cmd.routes = append(cmd.routes, tempRoute)
}

func (cmd *cmd) printHelp() {
	fmt.Println("Available commands:")

	for _, r := range cmd.routes {
		fmt.Printf("\n  %s\n", r.pattern)

		if r.description != "" {
			fmt.Printf("    Description: %s\n", r.description)
		}

		if r.help != "" {
			fmt.Printf("    Help: %s\n", r.help)
		}
	}
}
