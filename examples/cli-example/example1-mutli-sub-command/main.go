package main

import (
	"fmt"
	"os"
	"strings"

	"gofr.dev/pkg/gofr"
)

// helper function to parse flags manually
func getFlagValue(args []string, flag string, defaultValue string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return defaultValue
}

func main() {
	app := gofr.NewCMD()

	// -----------------------
	// Subcommand: hello
	// -----------------------
	app.SubCommand("hello", func(ctx *gofr.Context) (any, error) {
		args := os.Args[2:] // arguments after "hello"
		name := getFlagValue(args, "--name", "")
		if name == "" && len(args) > 0 && !strings.HasPrefix(args[0], "--") {
			name = args[0] // fallback to positional arg
		}
		if name == "" {
			name = "World"
		}
		ctx.Out.Println(fmt.Sprintf("Hello, %s!", name))
		return nil, nil
	})

	// -----------------------
	// Subcommand: goodbye
	// -----------------------
	app.SubCommand("goodbye", func(ctx *gofr.Context) (any, error) {
		args := os.Args[2:] // arguments after "goodbye"
		name := getFlagValue(args, "--name", "")
		if name == "" && len(args) > 0 && !strings.HasPrefix(args[0], "--") {
			name = args[0]
		}
		if name == "" {
			name = "Friend"
		}
		ctx.Out.Println(fmt.Sprintf("Goodbye, %s!", name))
		return nil, nil
	})

	// Run the CLI app
	app.Run()
}
