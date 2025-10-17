# CLI Applications 

GoFr provides a simple way to build command-line applications using `app.NewCMD()`. This creates standalone CLI tools without starting an HTTP server.

## Configuration
To configure logging for CLI applications, set the following environment variable:
- `CMD_LOGS_FILE`: The file path where CLI logs will be written. If not set, logs are discarded.


## Getting Started

Create a basic CLI application with subcommands:

```go
package main

import (
	"fmt"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.NewCMD()

	// Simple hello command
	app.SubCommand("hello", func(c *gofr.Context) (any, error) {
		return "Hello World!", nil
	}, gofr.AddDescription("Print hello message"))

	// Command with parameters
	app.SubCommand("greet", func(c *gofr.Context) (any, error) {
		name := c.Param("name")
		if name == "" {
			name = "World"
		}
		return fmt.Sprintf("Hello, %s!", name), nil
	})

	app.Run()
}
```

## Key GoFr CLI Methods

- **`app.NewCMD()`**: Initialize a CLI application
- **`app.SubCommand(name, handler, options...)`**: Add a subcommand
- **`gofr.AddDescription(desc)`**: Add help description
- **`gofr.AddHelp(help)`**: Add detailed help text
- **`ctx.Param(name)`**: Get command parameters
- **`ctx.Out.Println()`**: Print to stdout
- **`ctx.Logger`**: Access logging

## Running CLI Applications

Build and run your CLI:

```bash
go build -o mycli
./mycli hello
./mycli greet --name John
./mycli --help
```

## Example Commands

```bash
# Basic command
./mycli hello
# Output: Hello World!

# Command with parameter
./mycli greet --name Alice  
# Output: Hello, Alice!

# Help
./mycli --help
```

For more details, see the [sample-cmd example](../../../examples/sample-cmd).
