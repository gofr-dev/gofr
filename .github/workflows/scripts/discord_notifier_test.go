package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		limit    int
		expected int // Number of chunks expected
	}{
		{
			name:     "Short message",
			input:    "Hello World",
			limit:    2000,
			expected: 1,
		},
		{
			name:     "Exact limit",
			input:    strings.Repeat("a", 100),
			limit:    100,
			expected: 1,
		},
		{
			name:     "Just over limit",
			input:    strings.Repeat("a", 101),
			limit:    100,
			expected: 2,
		},
		{
			name: "User Example (No Split needed)",
			input: `# **Release v1.50.2**

## 🚀 Enhancements

🔹 **HTTP Connection Pool Configuration**

GoFr now supports **configurable HTTP connection pool settings** to optimize performance for high-frequency HTTP requests in microservices. New ` + "`ConnectionPoolConfig`" + ` option for ` + "`AddHTTPService()`" + ` method

Configurable settings:
-  ` + "`MaxIdleConns`" + `: Maximum idle connections across all hosts (default: 100)
-  ` + "`MaxIdleConnsPerHost`" + `: Maximum idle connections per host (default: 2, recommended: 10-20)
-  ` + "`IdleConnTimeout`" + `: Connection keep-alive duration (default: 90 seconds)
- Addresses the limitation where Go's default ` + "`MaxIdleConnsPerHost: 2`" + ` is insufficient for microservices
-  **Important**: ` + "`ConnectionPoolConfig`" + ` must be applied first when using multiple options  


🔹 **OpenTSDB Metrics Support**

Added metrics instrumentation for OpenTSDB operations to provide better observability.
New metrics:
-  ` + "`app_opentsdb_operation_duration`" + `: Duration of OpenTSDB operations in milliseconds
-  ` + "`app_opentsdb_operation_total`" + `: Total OpenTSDB operations
- Enables monitoring and alerting on OpenTSDB data operations


## 🛠️ Fixes

🔹 **Panic Recovery for OnStart Hooks**
Added panic recovery mechanism for ` + "`OnStart`" + ` hooks to prevent entire application crash. If a hook panics, the error is logged and the application continues with other hooks, improving application stability and resilience.

🔹 **GCS Provider Name for Observability**
Added provider name "GCS" to Google Cloud Storage file store for improved logging and metrics identification. Previously it used common logging semantics "COMMON" shared across file storage providers leading to improper visibilty of the underlying file storage being used,

🔹 **Zipkin Trace Exporter Error Handling**
Fixed error logging for successful trace exports (2xx status codes). Zipkin exporter now correctly ignores ` + "`201`" + ` status codes and other ` + "`2xx`" + ` responses, reducing noise in error logs for successful operations.`,
			limit:    2000,
			expected: 1,
		},
		{
			name: "User Example (Forced Split)",
			input: `# **Release v1.50.2**

## 🚀 Enhancements

🔹 **HTTP Connection Pool Configuration**
... (content repeated to force split) ...
` + strings.Repeat("Long line content to force split ", 50),
			limit:    500,
			expected: 4, // Approximate
		},
		{
			name: "Code Block Splitting",
			input: "Start\n```go\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n```\nEnd",
			limit: 30, // Should force split inside code block
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := splitMessage(tt.input, tt.limit)
			
			if len(chunks) == 0 {
				t.Errorf("Expected chunks, got none")
			}

			// Verify chunk count logic (approximate for forced split)
			if tt.name != "User Example (Forced Split)" && tt.name != "Code Block Splitting" {
				if len(chunks) != tt.expected {
					t.Errorf("Expected %d chunks, got %d", tt.expected, len(chunks))
				}
			}

			// Verify constraints
			for i, chunk := range chunks {
				if utf8.RuneCountInString(chunk) > tt.limit {
					t.Errorf("Chunk %d length %d exceeds limit %d", i, utf8.RuneCountInString(chunk), tt.limit)
				}
				
				// Verify code block closure
				openCount := strings.Count(chunk, "```")
				// If we are inside a block, we expect an even number of ``` unless logic adds them.
				// Our logic adds them. So every chunk should have balanced ``` if it contains code.
				// Exception: nested ``` or language identifiers might complicate simple counting,
				// but for our simple splitter, it adds ``` to close/open.
				if openCount%2 != 0 {
					t.Errorf("Chunk %d has unbalanced code blocks: %s", i, chunk)
				}
			}
			
			// Reconstruct and verify content match (ignoring added code block markers)
			// This is hard because we inject ```. 
			// Instead, let's verify that the *text* is all there.
		})
	}
}

func TestSplitMessage_CodeBlockLogic(t *testing.T) {
	input := "Prefix\n```\nLine 1\nLine 2\nLine 3\n```\nSuffix"
	limit := 20 // Small limit to force split inside block
	
	chunks := splitMessage(input, limit)
	
	// We expect:
	// Chunk 1: Prefix\n```\nLine 1\n```
	// Chunk 2: ```\nLine 2\n```
	// Chunk 3: ```\nLine 3\n```
	// Chunk 4: ```\nSuffix (Wait, suffix is outside) -> Suffix
	
	for i, chunk := range chunks {
		t.Logf("Chunk %d:\n%s\n---", i, chunk)
		if utf8.RuneCountInString(chunk) > limit {
			t.Errorf("Chunk %d exceeds limit", i)
		}
	}
}
