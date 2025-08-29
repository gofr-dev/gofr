# GoFr CLI Examples

This directory contains examples demonstrating how to build Command-Line Interface (CLI) applications using the GoFr framework.

⚠️ **Version Note**: These examples are designed for the current stable GoFr version where automatic argument parsing (`ctx.Args()` / `ctx.Flags()`) is not yet available. Arguments and flags are therefore parsed manually using `os.Args` and helper functions.

---

## Example 1: Multiple Subcommands with Manual Argument Parsing

**File**: `example1-mutli-sub-command/main.go`

This example demonstrates a robust GoFr CLI application supporting multiple subcommands (`hello` and `goodbye`). It showcases how to manually parse both flag-based and positional arguments using `os.Args` and a custom helper function.

### Purpose
- Define multiple subcommands: `hello` and `goodbye`.
- Handle `--name` flag and positional arguments (e.g., `hello John`, `goodbye Jane`).
- Provide default values if no argument is supplied for either subcommand.
- Output results using `ctx.Out.Println`.

### How to Run

First, ensure you have the `example1-mutli-sub-command/main.go` file created in this directory (content provided below).

Navigate to your GoFr project root and execute the following commands:

1.  **Run `hello` subcommand with default value (no arguments):**
    ```bash
    go run examples/cli-example/example1-mutli-sub-command/main.go hello
    # Output: Hello, World!
    ```

2.  **Run `hello` subcommand with a positional argument:**
    ```bash
    go run examples/cli-example/example1-mutli-sub-command/main.go hello John
    # Output: Hello, John!
    ```

3.  **Run `hello` subcommand with a flag argument:**
    ```bash
    go run examples/cli-example/example1-mutli-sub-command/main.go hello --name Jane
    # Output: Hello, Jane!
    ```

4.  **Run `goodbye` subcommand with default value (no arguments):**
    ```bash
    go run examples/cli-example/example1-mutli-sub-command/main.go goodbye
    # Output: Goodbye, Friend!
    ```

5.  **Run `goodbye` subcommand with a positional argument:**
    ```bash
    go run examples/cli-example/example1-mutli-sub-command/main.go goodbye Mark
    # Output: Goodbye, Mark!
    ```

6.  **Run `goodbye` subcommand with a flag argument:**
    ```bash
    go run examples/cli-example/example1-mutli-sub-command/main.go goodbye --name Susan
    # Output: Goodbye, Susan!
    ```

### Key Concepts Demonstrated
-   **`app.NewCMD()`**: Initializes a new GoFr CLI application.
-   **`app.SubCommand("name", handler)`**: Registers multiple subcommands with their respective handler functions.
-   **Manual Argument Parsing**: `os.Args` is used directly with a helper function (`getFlagValue`) to extract flag values (e.g., `--name`) and positional arguments within each subcommand's handler.
-   **`ctx.Out.Println()`**: Prints output directly to the standard CLI output stream.
-   **Subcommand Handler Signature:**
    ```go
    func(ctx *gofr.Context) (any, error)
    ```
    This is the required signature for all subcommand handlers, returning an optional result and an error.

---

## Example 2: Single Subcommand with Manual Argument Parsing

**File**: `example2-single-sub-command/main.go`

This example demonstrates a basic GoFr CLI application with a single subcommand (`hello`), focusing on manual argument parsing.

### Purpose
- Define a single subcommand: `hello`.
- Handle `--name` flag and positional arguments (e.g., `hello John`).
- Provide default values if no argument is supplied.
- Output results using `ctx.Out.Println`.

### How to Run

Navigate to your GoFr project root and execute:

1.  **Run with default value (no arguments):**
    ```bash
    go run examples/cli-example/example2-single-sub-command/main.go hello
    # Output: Hello, World!
    ```

2.  **Run with a positional argument:**
    ```bash
    go run examples/cli-example/example2-single-sub-command/main.go hello John
    # Output: Hello, John!
    ```

3.  **Run with a flag argument:**
    ```bash
    go run examples/cli-example/example2-single-sub-command/main.go hello --name Jane
    # Output: Hello, Jane!
    ```

### Key Concepts Demonstrated
-   **`app.NewCMD()`**: Initializes a new GoFr CLI application.
-   **`app.SubCommand("name", handler)`**: Registers a subcommand with its handler function.
-   **Manual Argument Parsing**: Use `os.Args` with a helper function (`getFlagValue`) to extract flag values (e.g., `--name`).
-   **`ctx.Out.Println()`**: Prints output directly to the standard CLI output stream.
-   **Subcommand Handler Signature:**
    ```go
    func(ctx *gofr.Context) (any, error)
    ```
    This is required for all subcommand handlers, returning an optional result and an error.