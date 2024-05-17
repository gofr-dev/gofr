# Filtered Logging

Filtered logging is a feature that allows you to mask sensitive data in your log messages. It helps protect personally identifiable information (PII) and other confidential data from being exposed in logs.

## Usage

To use filtered logging, you need to set the masking configuration in your `.env` file. The following configuration options are available:

- `LOGGER_MASKING_FIELDS`: A comma-separated list of field names that should be masked in log messages.

Here's an example of how you can configure the masking options in your `.env` file:

```
LOGGER_MASKING_FIELDS=password,email,creditCard
```

In this example, masking is enabled, and the fields "password", "email", and "creditCard" will be masked in the log messages.

Once you have configured the masking options in the `.env` file, the logger will automatically apply the masking based on the configuration. You don't need to call any additional functions or make any changes to your code.

Here's an example of how you can use the logger:

```go
package main

import (
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/logging"
)

func main() {
    app := gofr.New()

    // Use the logger
    logger := app.Logger()
    logger.Info("User login", map[string]interface{}{
        "username": "john.doe",
        "password": "secret123",
        "email":    "john.doe@example.com",
    })
}
```

In this example, the `New` function is called to create a new instance of the `App`. The logger is then retrieved using `app.Logger()`, and the log message is generated using `logger.Info()`.

When logging a message that contains sensitive data, the specified fields will be masked in the output. The masked fields will be replaced with asterisks (`*`) to protect the sensitive information.


## How It Works Internally

Internally, the filtered logging feature works as follows:

1. The `New` function of the `App` reads the masking configuration from the `.env` file.
2. If masking is enabled (`LOGGER_MASKING_FIELDS` has comma separated values), the function splits the `LOGGER_MASKING_FIELDS` value into a slice of field names.
3. The function removes any empty or whitespace-only field names from the slice.
4. The resulting slice of field names is passed to the `logging.SetMaskingFilters` function to set the masking filters.
5. When a log message is generated, the logger checks if any of the fields in the message match the masking filters.
6. If a field matches a masking filter, the value of that field is replaced with asterisks (`*`) to mask the sensitive data.
7. The masked log message is then outputted to the configured log destination (e.g., console, file).

## Points to Keep in Mind

When using the filtered logging feature, developers should keep the following points in mind:

1. The masking configuration is read from the `.env` file. Make sure to set the appropriate value for `LOGGER_MASKING_FIELDS` in your `.env` file.
2. The `LOGGER_MASKING_FIELDS` value should be a comma-separated list of field names. Ensure that the field names are specified correctly and match the field names in your log messages.
3. If a field name specified in `LOGGER_MASKING_FIELDS` does not exist in a log message, it will be ignored.
4. The masking process replaces the entire value of a sensitive field with asterisks (`*`). It does not preserve the original length or format of the value.
5. The filtered logging feature only masks the specified fields in the log messages. It does not provide encryption or secure storage of sensitive data.
6. Be cautious when specifying the masking fields to avoid masking non-sensitive data unintentionally.
