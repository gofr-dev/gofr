package serrors

type Error struct {
	cause              error
	message            string         // Human-readable message describing the error or status
	statusCode         string         // Primary error code representing the general error category agnostic of protocol
	subStatusCode      string         // For finer categorization (optional)
	level              Level          // Severity level (custom type, e.g., "INFO", "WARN", "ERROR", "FATAL")
	meta               map[string]any // Additional metadata or context (e.g., request IDs, timestamps, etc)
	retryable          bool           // Indicates if the operation can be safely retried
	externalStatusCode int            // External facing status code: could be HTTP status, gRPC status code, WebSocket close code, etc.
	externalMessage    string         // User friendly client facing error message
	ErrorSchema
}
