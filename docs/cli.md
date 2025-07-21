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
 