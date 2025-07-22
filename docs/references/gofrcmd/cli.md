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
 > **Note**  
> This document provides details about the **GoFr CLI**, a lightweight command-line utility built using the **GoFr framework itself**.  
> It serves as a practical example of how developers can leverage GoFr for building their own CLI applications.  
>  
> For existing reference documentation, see [GoFr CLI Docs](https://gofr.dev/docs/references/gofrcli).


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

	app.SubCommand("calc", "Performs basic arithmetic", func(ctx *gofr.Context) error {
		op := ctx.Flag("op")
		aStr := ctx.Flag("a")
		bStr := ctx.Flag("b")

		if op == "" || aStr == "" || bStr == "" {
			return errors.New("usage: ./calc-cli calc --op=<add|sub|mul|div> --a=<num1> --b=<num2>")
		}

		a, err1 := strconv.Atoi(aStr)
		b, err2 := strconv.Atoi(bStr)
		if err1 != nil || err2 != nil {
			return errors.New("both --a and --b must be valid integers")
		}

		var result int
		switch op {
		case "add":
			result = a + b
		case "sub":
			result = a - b
		case "mul":
			result = a * b
		case "div":
			if b == 0 {
				return errors.New("cannot divide by zero")
			}
			result = a / b
		default:
			return fmt.Errorf("unsupported operation: %s", op)
		}

		fmt.Printf("Result: %d\n", result)
		ctx.Logger.Infof("Performed %s on %d and %d", op, a, b)
		return nil
	})

	app.Run()
}
```

 >  **Note:**

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

 <p><strong>Snippet of Running Calculator.go</strong></p>
<img src="./calculator.png" alt="GoFr CLI calculator performing addition and division" width="500"/>

 **Ending Note:**
   
> This CLI app demonstrates how to structure and run basic command-line utilities using Go. You can enhance it further by adding error handling, more operations, or using third-party CLI libraries like Cobra for advanced features.

>  **Did you know?**  
> The GoFr CLI itself is a utility built entirely using the **GoFr framework**.  
> This demonstrates how GoFr can be used not just for web services, but also for building powerful command-line tools.
