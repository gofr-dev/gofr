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
type ErrCommandNotFound struct {
	Command string
}

func (e ErrCommandNotFound) Error() string {
	return fmt.Sprintf("'%s' is not a valid command.", e.Command)
}

func (cmd *cmd) Run(c *container.Container) {
	args := os.Args[1:] // First one is command itself
	subCommand, showHelp, firstArg := parseArgs(args)

	if showHelp && subCommand == "" {
		cmd.printHelp()
		return
	}

	r := cmd.handler(subCommand)
	ctx := newCMDContext(&cmd2.Responder{}, cmd2.NewRequest(args), c, cmd.out)

	commandForError := getCommandForError(subCommand, firstArg)

	if cmd.noCommandResponse(r, ctx, commandForError) {
		return
	}

	if showHelp {
		cmd.out.Println(r.help)
		return
	}

	ctx.responder.Respond(r.handler(ctx))
}

// parseArgs parses command line arguments and returns subCommand, showHelp flag, and firstArg.
func parseArgs(args []string) (subCommand string, showHelp bool, firstArg string) {
	subCommand = ""
	showHelp = false

	for _, a := range args {
		if a == "" {
			continue // This takes care of cases where subCommand has multiple spaces in between.
		}

		if a == "-h" || a == "--help" {
			showHelp = true

			continue
		}

		if firstArg == "" {
			firstArg = a
		}

		if a[0] != '-' {
			subCommand = subCommand + " " + a
		}
	}

	return subCommand, showHelp, firstArg
}

// getCommandForError returns the command string to use in error messages.
func getCommandForError(subCommand, firstArg string) string {
	commandForError := strings.TrimSpace(subCommand)

	if commandForError == "" && firstArg != "" {
		commandForError = firstArg
	}

	return commandForError
}

// noCommandResponse responds with error when no route with the given subcommand is not found or handler is nil.
func (cmd *cmd) noCommandResponse(r *route, ctx *Context, subCommand string) bool {
	if r == nil {
		ctx.responder.Respond(nil, ErrCommandNotFound{Command: strings.TrimSpace(subCommand)})
		fmt.Println()
		cmd.printHelp()

		return true
	}

	if r.handler == nil {
		ctx.responder.Respond(nil, ErrCommandNotFound{Command: strings.TrimSpace(subCommand)})

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

	var maxPatternLen, maxDescLen int

	for _, r := range cmd.routes {
		if len(r.pattern) > maxPatternLen {
			maxPatternLen = len(r.pattern)
		}

		if len(r.description) > maxDescLen {
			maxDescLen = len(r.description)
		}
	}

	for _, r := range cmd.routes {
		fmt.Printf("  %-*s  %-*s", maxPatternLen, r.pattern, maxDescLen, r.description)

		if r.help != "" {
			fmt.Printf("  Help: %s", r.help)
		}

		fmt.Println()
	}
}
