package surrealdb

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/surrealdb/surrealdb.go/pkg/models"
	"go.opentelemetry.io/otel/trace"
)

var rgx = regexp.MustCompile(`\s+`)

// clean takes a string query as input and performs two operations to clean it up:
// 1. It replaces multiple consecutive whitespace characters with a single space.
// 2. It trims leading and trailing whitespace from the string.
// The cleaned-up query string is then returned.
func clean(query string) string {
	// Replace multiple consecutive whitespace characters with a single space
	query = rgx.ReplaceAllString(query, " ")

	// Trim leading and trailing whitespace from the string
	query = strings.TrimSpace(query)

	return query
}

type QueryLog struct {
	Query         string     `json:"query"`                // The query executed.
	OperationName string     `json:"operationName"`        // The operation name
	Duration      int64      `json:"duration"`             // Execution time in microseconds.
	Namespace     string     `json:"namespace"`            // The namespace of the query.
	Database      string     `json:"database"`             // The database the query was executed on.
	ID            any        `json:"id"`                   // The ID of the affected items.
	Data          any        `json:"data"`                 // The data affected or retrieved.
	Filter        any        `json:"filter,omitempty"`     // Optional filter applied to the query.
	Update        any        `json:"update,omitempty"`     // Optional update data for the query.
	Collection    string     `json:"collection,omitempty"` // Optional collection affected.\
	Span          trace.Span `json:"span,omitempty"`       // Optional tracing span associated with the query.
}

const defaultValue = "default"

// PrettyPrint outputs a formatted string representation of the QueryLog.
func (ql *QueryLog) PrettyPrint(writer io.Writer) {
	// Set default values for nil fields
	if ql.Filter == nil {
		ql.Filter = ""
	}

	if ql.ID == nil {
		ql.ID = ""
	}

	if ql.Update == nil {
		ql.Update = ""
	}

	if ql.Database == "" {
		ql.Database = defaultValue
	}

	if ql.Namespace == "" {
		ql.Namespace = defaultValue
	}

	// Format string with proper color codes and positioning
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s:%s \u001B[38;5;8m%-32s\u001B[0m\n",
		clean(ql.OperationName),
		"SRLDB",
		ql.Duration,
		ql.Database,
		ql.Namespace,
		clean(ql.Query),
	)
}

func isAdministrativeOperation(query string) bool {
	return strings.HasPrefix(query, "DEFINE") ||
		strings.HasPrefix(query, "REMOVE") ||
		strings.Contains(query, "NAMESPACE") ||
		strings.Contains(query, "DATABASE")
}

// isCustomNil checks for CustomNil type.
func isCustomNil(result any) bool {
	_, ok := result.(models.CustomNil)
	return ok
}
