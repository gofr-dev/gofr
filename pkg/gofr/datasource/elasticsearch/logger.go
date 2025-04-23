package elasticsearch

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var whitespaceRegex = regexp.MustCompile(`\s+`)

// Logger interface with required methods for Elasticsearch logging.
type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Log(args ...any)
	Logf(pattern string, args ...any)
	Error(args ...any)
	Errorf(pattern string, args ...any)
}

// QueryLog holds information about an Elasticsearch operation for structured logging.
type QueryLog struct {
	Operation  string   `json:"operation"`             // e.g., "search", "index-document"
	Indices    []string `json:"indices,omitempty"`     // target indices
	DocumentID string   `json:"document_id,omitempty"` // document ID for doc operations
	Target     string   `json:"target,omitempty"`      // custom context (e.g., index/alias)
	Request    any      `json:"request,omitempty"`     // raw query or body payload
	Duration   int64    `json:"duration"`              // duration in microseconds
}

// PrettyPrint formats the QueryLog and emits a colored, structured log line.
func (ql *QueryLog) PrettyPrint(logger Logger) {
	var payload string

	if ql.Request != nil {
		if data, err := json.Marshal(ql.Request); err == nil {
			payload = string(data)
		}
	}

	// Build context parts: indices, document ID, target, status
	ctxParts := []string{}
	if len(ql.Indices) > 0 {
		ctxParts = append(ctxParts, strings.Join(ql.Indices, ","))
	}

	if ql.DocumentID != "" {
		ctxParts = append(ctxParts, ql.DocumentID)
	}

	if ql.Target != "" {
		ctxParts = append(ctxParts, ql.Target)
	}

	contextStr := clean(strings.Join(ctxParts, " "))

	// Clean operation and payload strings
	op := clean(ql.Operation)
	pl := clean(payload)

	// Format: [Operation] [ELASTIC] Duration µs Context Payload
	formatted := fmt.Sprintf(
		"\u001B[38;5;8m%-15s \u001B[38;5;208mELASTIC\u001B[0m %8dµs %-15s %s",
		op, ql.Duration, contextStr, pl,
	)
	logger.Debug(formatted)
}

// clean replaces consecutive whitespace with a single space and trims.
func clean(s string) string {
	s = whitespaceRegex.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
