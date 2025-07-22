# GoFr CLI Documentation

GoFr supports building command-line interface (CLI) applications using the same architecture as its web services. To enable CLI mode, initialize your application with gofr.NewCMD() instead of gofr.New(). This activates CLI-specific behavior, automatically configuring logging, configuration management, and context handling for command-line workflows.

 ---

## CLI Application Structure

- *Initialization:* Use gofr.NewCMD() to set up your CLI application.
- *Command Registration:* Define each CLI command with a name and a handler function.
- *Context:* Each command handler receives a context object for accessing configuration, logging, arguments, and environment variables.

 ---

## Logging

- *Output:* Logs are output to stdout by default.
- *Log Levels:* Supported levels include DEBUG, INFO, WARN, ERROR, and FATAL.
- *Configuration:* Set the log level using the environment variable LOG_LEVEL. Example values: DEBUG, INFO, ERROR.
- *Format:* Each log message includes a timestamp, level, and message content.
- *Customization:* By default, logs go to stdout. Advanced users can redirect logs using Go's logging configuration tools if required, though this is not necessary for standard usage.

 ---

## Configuration and Environment

- GoFr CLI applications accept configuration and environment variables the same way as its web framework mode.
- Common variables for CLI apps include LOG_LEVEL and GOFR_ENV.
- All configuration is accessible through the context provided to each command handler.

 ---

## Best Practices

- Register clear, descriptive command names for each CLI function.
- Use environment variables to manage configuration and logging.
- Prefer using the provided logger over direct output for consistency and structure.
- Structure commands with single-responsibility to simplify usage and maintenance.

 ---

 ## Sample CLI App in GoFr

Here's a simple yet useful CLI calculator built with GoFr. It supports operations like `add`, `sub`, `mul`, and `div`.

### Code Example

```go
package main

import (
    "errors"
    "fmt"
    "strconv"

    "gofr.dev/pkg/gofr"
)

func main() {
    app := gofr.NewCMD()

    app.Command("calc", "Performs basic arithmetic", func(ctx *gofr.Context) error {
        if len(ctx.Args) < 3 {
            return errors.New("usage: calc <add|sub|mul|div> <num1> <num2>")
        }

        op := ctx.Args[0]
        num1, err1 := strconv.Atoi(ctx.Args[1])
        num2, err2 := strconv.Atoi(ctx.Args[2])

        if err1 != nil || err2 != nil {
            return errors.New("both arguments must be valid integers")
        }

        switch op {
        case "add":
            fmt.Println("Result:", num1+num2)
        case "sub":
            fmt.Println("Result:", num1-num2)
        case "mul":
            fmt.Println("Result:", num1*num2)
        case "div":
            if num2 == 0 {
                return errors.New("cannot divide by zero")
            }
            fmt.Println("Result:", num1/num2)
        default:
            return fmt.Errorf("unsupported operation: %s", op)
        }

        ctx.Logger.Info("Calculation completed successfully")
        return nil
    })

    app.Run()
}

 ---

 >  **Note**

> Create the file: Save the above code in a file named `calculator.go`.  
>  
>  **Set Up Go:** Ensure Go is installed on your system:  
> ```bash
> go version
> ```  
>  
>  **Build the CLI App:**  
> Open terminal → navigate to the directory containing `calculator.go` → run:  
> ```bash
> go build -o calculator calculator.go
> ```  
> This creates an executable file named `calculator` (or `calculator.exe` on Windows).  
>  
>  **Run the CLI App:**  
> ```bash
> ./calculator calc add 4 6     # Output: Result: 10  
> ./calculator calc sub 10 3    # Output: Result: 7  
> ./calculator calc mul 3 5     # Output: Result: 15  
> ./calculator calc div 12 3    # Output: Result: 4
> ```

 **Ending Note:**
   
> This CLI app demonstrates how to structure and run basic command-line utilities using Go. You can enhance it further by adding error handling, more operations, or using third-party CLI libraries like Cobra for advanced features.