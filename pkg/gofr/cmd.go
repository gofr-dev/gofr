package gofr

import (
	"fmt"
	"os"
	"strings"

	cmd2 "gofr.dev/pkg/gofr/cmd"
	"gofr.dev/pkg/gofr/cmd/terminal"
	"gofr.dev/pkg/gofr/container"
)

type cmd struct {
	routes []route
	out    terminal.Output
}

type route struct {
	pattern     string
	handler     Handler
	description string
	help        string
}

// Options is a function type used to configure a route in the command handler.
type Options func(c *route)

// ErrCommandNotFound is an empty struct used to represent a specific error when a command is not found.
type ErrCommandNotFound struct{}

func (ErrCommandNotFound) Error() string {
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
	ctx := newCMDContext(&cmd2.Responder{}, cmd2.NewRequest(args), c, cmd.out)

	// handling if route is not found or the handler is nil
	if cmd.noCommandResponse(r, ctx) {
		return
	}

	if showHelp {
		cmd.out.Println(r.help)
		return
	}

	// Execute command with panic recovery
	func() {
		defer NewRecoveryHandler(c.Logger, "cmd:"+r.pattern).Recover()
		ctx.responder.Respond(r.handler(ctx))
	}()
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
	path = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(path, "--"), "-"))

	// Iterate over the routes to find a matching handler
	for _, r := range cmd.routes {
		if strings.HasPrefix(path, r.pattern) {
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

// addRoute adds a new route to cmd's list of routes.
func (cmd *cmd) addRoute(pattern string, handler Handler, options ...Options) {
	// Define restricted characters
	restrictedChars := "$^"

	// Check if the pattern contains any restricted characters
	for i := 0; i < len(pattern); i++ {
		if strings.ContainsRune(restrictedChars, rune(pattern[i])) {
			fmt.Println("found a restricted character in the command while registering with GoFr:", pattern[i])
			return
		}
	}

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
