# Filtered Logging

Filtered logging is a feature that allows you to mask sensitive data in your log messages. It helps protect personally identifiable information (PII) and other confidential data from being exposed in logs.

## Usage

To use filtered logging, you need to create a logger instance with a filter that specifies the fields to mask. Here's an example of how to set up and use filtered logging:

```go
package main

import (
	"gofr.dev/pkg/gofr/logging"
)

func main() {
	// Create a custom filter with specific masking fields
	filter := &logging.DefaultFilter{
		MaskFields: []string{"password", "email", "creditCard"},
	}

	// Create a logger with the custom filter
	logger := logging.NewLogger(logging.INFO, filter)

	// Log a message with sensitive data
	logger.Info("User login", map[string]interface{}{
		"username":   "john.doe",
		"password":   "secret123",
		"email":      "john.doe@example.com",
		"creditCard": "1234-5678-9012-3456",
	})

	// Output:
	// {"level":"INFO","time":"2023-06-08T10:30:00Z","message":{"username":"john.doe","password":"**********","email":"*********************","creditCard":"****************"},"gofrVersion":"1.0.0"}
}
```

In this example, we create a custom filter (`DefaultFilter`) and specify the fields we want to mask (`password`, `email`, `creditCard`). Then, we create a logger instance with the custom filter using `logging.NewLogger()`.

When logging a message that contains sensitive data, the specified fields will be masked in the output. The masked fields will be replaced with asterisks (`*`) to protect the sensitive information.

## Enabling and Disabling Masking

By default, masking is enabled when you create a filter. However, you can easily enable or disable masking as needed.

To disable masking, set the `EnableMasking` flag to `false` on the filter:

```go
filter.EnableMasking = false
```

When masking is disabled, the log messages will include the actual values of the fields without any masking.

To enable masking again, set the `EnableMasking` flag to `true`:

```go
filter.EnableMasking = true
```

With masking enabled, the specified fields will be masked in the log output.

## Customizing Masking Fields

You can customize the fields that you want to mask by modifying the `MaskFields` slice on the filter. Add the field names that you want to mask to the slice:

```go
filter := &logging.DefaultFilter{
	MaskFields: []string{"password", "email", "creditCard", "ssn"},
}
```

In this example, the `password`, `email`, `creditCard`, and `ssn` fields will be masked in the log output.

## Handling Pointers

The filtered logging feature handles pointers correctly. If a field is a pointer, the masking will be applied to the underlying value.

For example, if you have a struct with pointer fields:

```go
type User struct {
	Username *string
	Password *string
	Email    *string
}
```

The filtered logging will mask the values of the `Username`, `Password`, and `Email` fields even though they are pointers.

