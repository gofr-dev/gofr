# Building CLI Applications with GoFr

GoFr provides a robust and straightforward way to build **command-line applications**. By leveraging `app.NewCMD()`, you gain access to a powerful framework that simplifies the process of defining custom commands, handling flags, managing configuration, and orchestrating execution flows for your CLI tools.

This approach offers a clear separation from HTTP services, allowing you to focus purely on command-line interactions and logic.

## Table of Contents

-   [Introduction](#introduction)
-   [Getting Started: Your First GoFr CLI App](#getting-started-your-first-gofr-cli-app)
    -   [Understanding the `Handler` Function](#understanding-the-handler-function)
    -   [Using Command Options: `AddDescription` and `AddHelp`](#using-command-options-adddescription-and-addhelp)
-   [Running the CLI Application](#running-the-cli-application)
-   [Demo Run](#demo-run)
-   [Error Handling with Exit Codes](#error-handling-with-exit-codes)
-   [Logging in CLI Applications](#logging-in-cli-applications)
    -   [Log Levels](#log-levels)
    -   [Using the Logger in Handlers](#using-the-logger-in-handlers)
    -   [Controlling Log Level via Environment Variable](#controlling-log-level-via-environment-variable)
-   [Testing CLI Commands](#testing-cli-commands)
-   [Comparison Table: GoFr HTTP vs. CLI Applications](#comparison-table-gofr-http-vs-cli-applications)
-   [Best Practices for GoFr CLI Applications](#best-practices-for-gofr-cli-applications)
-   [Distinction: `gofr-cli` vs. `app.NewCMD()`](#distinction-gofr-cli-vs-appnewcmd)
-   [Navigation Note](#navigation-note)

## Introduction

In GoFr, applications are typically initialized in two primary ways:

-   **`app.New()`**: This function is dedicated to creating **HTTP services**, such as REST APIs, web applications, and microservices. It automatically sets up an HTTP server, handles routing for web requests, integrates middleware, and provides a full suite of HTTP-specific functionalities for building networked applications.

-   **`app.NewCMD()`**: This function is specifically designed for building **standalone command-line interface (CLI) applications**. Unlike `app.New()`, it does not initialize or start an HTTP server. Instead, `app.NewCMD()` provides the necessary infrastructure to define distinct subcommands, associate them with handler functions, parse command-line arguments and flags, and execute logic directly from the terminal. It's ideal for automation, scripting, and developer tools.

⚠️ **Version Note**: This example works with the current stable GoFr version in which automatic argument parsing (`ctx.Args()` / `ctx.Flags()`) is not yet available. Therefore, arguments and flags are parsed manually using `os.Args`. Future GoFr releases may provide built-in flag parsing, but this approach is fully functional for open-source use and ensures compatibility.

## Getting Started: Your First GoFr CLI App

Let's begin by creating a simple CLI application that performs basic arithmetic operations (addition and subtraction) using subcommands.

First, ensure your Go module is initialized and GoFr is added as a dependency:

```bash
go mod init github.com/example/mycli
go get gofr.dev
```

Next, create a `main.go` file (e.g., in your project's root) with the following content:

```go
package main

import (
	"fmt"
	"os"     // Required for os.Args to manually parse arguments
	"strings" // Required for string manipulation in argument parsing

	"gofr.dev/pkg/gofr"
)

// helper function to parse flags manually
// This function is necessary because this GoFr version does not support automatic flag parsing
// via `ctx.Args()` or `ctx.Flags()`.
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
		// In this GoFr version, argument parsing is done manually.
		// os.Args[0] is the executable name, os.Args[1] is the subcommand ("hello").
		// So, actual arguments for the subcommand start from index 2.
		args := os.Args[2:] 
		
		// Attempt to get "name" from a flag like "--name John"
		name := getFlagValue(args, "--name", "")

		// If no --name flag, fallback to a positional argument (e.g., "hello John")
		if name == "" && len(args) > 0 && !strings.HasPrefix(args[0], "--") {
			name = args[0] 
		}

		if name == "" {
			name = "World" // Default value if no argument is provided
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

	// Run the CLI application. This will parse arguments and execute the matching subcommand.
	app.Run()
}
```
**Find this example and more in the `examples/cli-example` directory.**

### Understanding the `Handler` Function

The handler function for `app.SubCommand` has the signature `func(ctx *gofr.Context) (any, error)`.

-   **`ctx *gofr.Context`**: This is the GoFr context object, which provides access to:
    -   **Command-line parameters (manual parsing)**: In this GoFr version, `ctx.Args()` and `ctx.Flags()` are not available for automatic argument parsing. Instead, you will manually parse `os.Args` within your handler. You can use helper functions like `getFlagValue(args []string, flag string, defaultValue string)` (as shown in the example) to extract flag values. Positional arguments also need to be parsed from `os.Args`. The example `main.go` demonstrates handling both `--name` flags and positional arguments.
    -   **Logger**: `c.Logger` for structured logging.
    -   **Standard Output**: `ctx.Out.Println()` can be used to print directly to standard output.
    -   **Other dependencies**: Access to databases, services, and configuration registered with the GoFr application.
-   **`any`**: The return value that will be printed to `stdout` upon successful execution of the command.
-   **`error`**: If the handler returns a non-nil error, the application will print the error message to `stderr` and exit with a non-zero status code. This is crucial for scripting.

### Using Command Options: `AddDescription` and `AddHelp`

When defining subcommands, you can provide additional `Options` to enhance the user experience and make your CLI tool self-documenting:

-   **`gofr.AddDescription(description string)`**: Provides a brief, one-line summary of what the subcommand does. This description is displayed when a user runs the main command without any arguments or with a global `--help` flag.
-   **`gofr.AddHelp(helpString string)`**: Offers more detailed usage information specifically for that subcommand. This comprehensive help text is shown when a user runs the subcommand with the `--help` or `-h` flag (e.g., `./mycli add --help`).

These options are crucial for creating user-friendly and discoverable CLI tools.

## Running the CLI Application

To make your CLI application executable, you first need to build it:

```bash
go build -o mycli
```
This command compiles your `main.go` file and creates an executable named `mycli` (or `mycli.exe` on Windows) in your current directory.

## Demo Run

Here's a sample run demonstrating how the CLI operates, including how to access help and use different argument styles:

```bash
# Run the 'hello' subcommand with no arguments (uses default)
$ ./mycli hello
Hello, World!

# Run the 'hello' subcommand with a positional argument
$ ./mycli hello Jane
Hello, Jane!

# Run the 'hello' subcommand with a --name flag
$ ./mycli hello --name John
Hello, John!

# Run the 'goodbye' subcommand with no arguments (uses default)
$ ./mycli goodbye
Goodbye, Friend!

# Run the 'goodbye' subcommand with a positional argument
$ ./mycli goodbye Mark
Goodbye, Mark!

# Run the 'goodbye' subcommand with a --name flag
$ ./mycli goodbye --name Susan
Goodbye, Susan!

# Access global help (shows descriptions of all subcommands)
$ ./mycli --help
Available commands:

  hello
    Description: 

  goodbye
    Description: 
```
Note: Both positional arguments (e.g., "hello Jane") and flags (e.g., "hello --name John") are supported, but flags are parsed manually using the helper function `getFlagValue`.

This demo illustrates how arguments are passed, how subcommands are executed, and how built-in help features work, reflecting the manual parsing approach.

## Error Handling with Exit Codes

For robust CLI applications, it's essential to communicate success or failure through standard exit codes. A `0` exit code indicates success, while any non-zero code signals an error. GoFr automatically handles returning a non-zero exit code if your handler returns an `error`.

However, for specific error conditions like invalid arguments, you might want to explicitly exit with a particular code, such as `2` for incorrect usage. You can achieve this within your handler:

```go
// Example of a handler explicitly exiting on validation error
app.SubCommand("mycommand", func(c *gofr.Context) (any, error) {
    // Note: In this GoFr version, argument parsing is manual.
    // You would use os.Args and your getFlagValue helper here.
    // For demonstration, let's assume 'required_arg' is parsed.
    requiredArg := getFlagValue(os.Args[2:], "--required_arg", "") 
    
    if requiredArg == "" {
        fmt.Fprintln(os.Stderr, "Error: 'required_arg' is missing.")
        os.Exit(2) // Exit with code 2 for incorrect usage
    }
    // ... command logic ...
    return "Command executed successfully", nil
})
```

By default, if a handler returns an error, GoFr will log the error to `stderr` and exit with `1`. Using `os.Exit()` allows for more granular control over exit codes, which is vital for scripting.

## Logging in CLI Applications

In GoFr CLI applications, logging output is directed to `stdout` by default. This makes it easy to integrate your CLI output into scripting pipelines or redirect logs to files. You can control the verbosity of your application's logs using the `GOFR_LOG_LEVEL` environment variable.

### Log Levels

GoFr supports standard logging levels:
-   `DEBUG`: Detailed information, typically of interest only when diagnosing problems.
-   `INFO`: Informational messages that highlight the progress of the application at a coarse-grained level.
-   `WARN`: Potentially harmful situations.
-   `ERROR`: Error events that might still allow the application to continue running.
-   `FATAL`: Severe error events that will presumably lead the application to abort.

### Using the Logger in Handlers

You can access the logger via the `gofr.Context` object within your subcommand handlers:

```go
// Inside a subcommand handler
func(c *gofr.Context) (any, error) {
    c.Logger.Debug("This is a debug message.")
    // Note: c.Params() might not accurately reflect manually parsed arguments.
    // Consider logging the manually parsed arguments directly.
    c.Logger.Infof("Processing command. Manually parsed arguments would be used here.") 
    c.Logger.Warn("Potential issue detected.")
    c.Logger.Errorf("Failed to complete task: %s", "reason for error")
    // c.Logger.Fatal will terminate the application after logging
    return "Task completed", nil
}
```

### Controlling Log Level via Environment Variable

Set the `GOFR_LOG_LEVEL` environment variable before running your CLI application. Example output will vary based on the messages logged in your actual handler:

```bash
# Set log level to DEBUG to see all messages
# For commands that use manual argument parsing, ensure your handler logic supports it.
$ GOFR_LOG_LEVEL=DEBUG ./mycli hello --name TestUser
# Expected output will depend on your handler's logging, e.g.:
{"level":"INFO","msg":"Hello, TestUser!"}

# Set log level to INFO (default)
$ GOFR_LOG_LEVEL=INFO ./mycli goodbye AnotherUser
# Expected output:
{"level":"INFO","msg":"Goodbye, AnotherUser!"}

# Set log level to ERROR to only see error and fatal messages
$ GOFR_LOG_LEVEL=ERROR ./mycli hello
# Expected output (assuming no error in default hello):
{"level":"INFO","msg":"Hello, World!"}
```

## Testing CLI Commands

GoFr emphasizes testability, and CLI command handlers are no exception. You can easily write unit tests for your subcommand handlers by mocking the `gofr.Context` and asserting the output or returned errors.

Here's an example of how you might test a subcommand handler using the manual argument parsing approach:

```go
// In a file like 'main_test.go' alongside your main.go
package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/cmd/cmdtest" // Utility for testing CLI commands
)

// Define the helloHandler function separately for easier testing
func helloHandler(ctx *gofr.Context) (any, error) {
    // For testing, cmdtest.NewContext can simulate arguments for ctx.Param.
    // In a real application, you would manually parse os.Args.
    name := ctx.Param("name") 
    if name == "" {
        name = "World"
    }
    ctx.Out.Println(fmt.Sprintf("Hello, %s!", name))
    return nil, nil
}

func TestHelloSubcommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string // Arguments to simulate for the subcommand
		expected string   // Expected output to ctx.Out.Println
		err      string
	}{
		{
			name:     "successful hello with name flag",
			args:     []string{"--name", "TestUser"},
			expected: "Hello, TestUser!",
			err:      "",
		},
		{
			name:     "successful hello with positional argument",
			args:     []string{"TestPositional"}, // cmdtest.NewContext maps this to "name" param
			expected: "Hello, TestPositional!",
			err:      "",
		},
		{
			name:     "hello with no arguments (default)",
			args:     []string{},
			expected: "Hello, World!",
			err:      "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// cmdtest.NewContext provides a mock context for testing CLI commands.
			// For ctx.Param to work correctly in tests, you need to ensure cmdtest.NewContext
			// is set up to mimic your manual parsing logic for os.Args.
			// This often means providing mock parameters that `ctx.Param` can then retrieve.
			c := cmdtest.NewContext(tc.args) 

            // Example of how you might mock ctx.Param behavior within a test,
            // though the exact implementation depends on cmdtest.NewContext capabilities.
            // If cmdtest.NewContext directly parses string slices into parameters,
            // then this manual mapping below might not be necessary.
            
            // Let's assume cmdtest.NewContext (or a custom test helper)
            // correctly translates `tc.args` to what `ctx.Param` would return
            // based on the `getFlagValue` logic in your main application.
            // For instance, if `args: []string{"--name", "TestUser"}`, then `c.Param("name")` should return "TestUser".
            // If `args: []string{"TestPositional"}`, then `c.Param("name")` should return "TestPositional".
            // And if `args: []string{}`, `c.Param("name")` should return "".

			_, err := helloHandler(c) // We are primarily interested in the side-effect (Println) and error

			if tc.err != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.err)
			} else {
				assert.NoError(t, err)
				// Asserting output can be done by capturing ctx.Out, but for simplicity
				// we'll assume direct output from the handler is covered by integration tests.
				// If you want to test output, you would need to mock ctx.Out to a buffer.
			}
		})
	}
}
```
This test uses `cmdtest.NewContext` to create a mock context, allowing you to pass arguments programmatically and assert the output or errors from your handler. Remember to integrate `strconv` for argument parsing if your handler expects integer inputs from flags, and explicitly handle `os.Args` as demonstrated in the example `main.go` and `getFlagValue` helper. When using `cmdtest.NewContext`, ensure its setup accurately reflects how your `main.go` manually parses arguments into parameters accessible via `ctx.Param`.

## Comparison Table: GoFr HTTP vs. CLI Applications

| Feature              | GoFr HTTP Applications (`app.New()`)           | GoFr CLI Applications (`app.NewCMD()`)              |
| :------------------- | :--------------------------------------------- | :-------------------------------------------------- |
| **Primary Purpose**  | Building web services, APIs, and microservices | Creating command-line tools and scripts             |
| **Server**           | Starts an HTTP server (listens on ports)       | Does NOT start an HTTP server                       |
| **Routing/Commands** | Handles HTTP routes (GET, POST, PUT, DELETE)   | Handles subcommands and their respective handlers   |
| **Request Handling** | Processes HTTP requests, generates responses   | **Manually parses command-line arguments and flags via `os.Args`** |
| **Input/Output**     | HTTP requests (JSON, XML), HTTP responses      | Terminal arguments (`os.Args`), `stdout`/`stderr`   |
| **Concurrency**      | Manages concurrent HTTP requests               | Typically executes one command at a time (sequential) |
| **Use Cases**        | Web APIs, backend services for web/mobile UIs  | Automation scripts, data processing, system utilities, developer tools |
| **Configuration**    | Configures HTTP ports, database connections, etc. | Configures logging levels, file paths, etc. (often via environment variables) |

## Best Practices for GoFr CLI Applications

Adhering to best practices ensures your CLI tools are robust, user-friendly, and maintainable.

*   **Keep Commands Modular and Focused**:
    *   Design each subcommand to perform a single, well-defined task. Avoid creating "god" commands that try to do too many things. This improves clarity, makes the command easier to test, and reduces the likelihood of bugs.
    *   For complex workflows, consider chaining multiple simple commands rather than building one monolithic command.

*   **Provide Helpful Output and Exit Codes**:
    *   **Informative Output**: Ensure your CLI provides clear, concise, and actionable feedback to the user. Use `fmt.Println`, `ctx.Out.Println` or `c.Logger.Info` for general output. For structured data, consider JSON or table formats.
    *   **Meaningful Exit Codes**: Use standard Unix exit codes:
        *   `0`: Indicates successful execution.
        *   `1`: General error, uncategorized failure.
        *   `2`: Incorrect usage, e.g., missing arguments or invalid flags.
        *   Other non-zero codes: Can be used for specific error conditions (e.g., file not found, permission denied). This is crucial for scripting and automation, allowing other programs to react to your CLI's outcome. You can use `os.Exit(code)` to set the exit code explicitly.
    *   Direct user-facing error messages to `stderr` (`fmt.Fprintln(os.Stderr, "Error message")`) to separate them from successful command output.

*   **Add Tests for Command Handlers**:
    *   Just like HTTP handlers, your CLI subcommand handlers contain business logic that needs to be thoroughly tested. Write unit and integration tests for each handler to ensure its correctness and reliability.
    *   Leverage GoFr's `cmdtest` utilities to easily mock the `gofr.Context` for testing, keeping in mind the need for manual argument parsing in your current setup.
    *   Mock external dependencies (databases, APIs) to isolate your handler logic during unit tests.

*   **Robust Error Handling**:
    *   Always handle potential errors gracefully within your command handlers. Return a meaningful `error` from your handler function, which GoFr will then log or display.
    *   **Wrap errors with context**: Use `fmt.Errorf("descriptive message: %w", originalErr)` to add context to errors as they propagate up, making debugging much easier. This is demonstrated in the `Getting Started` example.
    *   For user-facing errors, provide clear messages that guide the user on how to resolve the issue.

## Distinction: `gofr-cli` vs. `app.NewCMD()`

It's important to clarify the relationship between the official `gofr-cli` tool and the custom CLI applications you build using `app.NewCMD()`:

⚡ **Important Clarification**

-   The official **`gofr-cli`** tool ([https://gofr.dev/docs/references/gofrcli](https://gofr.dev/docs/references/gofrcli)) is a powerful, pre-built command-line utility distributed by the GoFr team. It helps GoFr developers with common tasks such as project initialization (`gofr-cli init`), database migrations (`gofr-cli migrate`), and more.
-   **Crucially, the `gofr-cli` tool itself is implemented using `app.NewCMD()`**. This serves as a real-world example of how GoFr’s CLI system can be used to build production-ready command-line tools. When you execute commands like `gofr-cli migrate` or `gofr-cli init`, you are interacting with an application built *with* GoFr's CLI framework under the hood.

👉 As a developer, you can leverage the exact same approach (`app.NewCMD()`) to build your **own custom CLI tools**. This allows you to create specialized utilities tailored to your project's unique automation, DevOps tasks, data processing needs, and more.

### Example Use Cases for Your Own `app.NewCMD()` Tools

*   **Database migration tools**: Custom scripts to manage schema changes or data seeding.
*   **Developer productivity scripts**: Automating repetitive development tasks specific to your codebase.
*   **Infrastructure automation**: CLI tools to deploy, configure, or monitor your services.
*   **Domain-specific CLI utilities**: Tools for managing specific business logic, data imports/exports, or system interactions.

By integrating `app.NewCMD()`, you benefit from GoFr's consistent structure, flexible logging, and robust error handling, enabling you to build powerful and reliable command-line utilities with ease.