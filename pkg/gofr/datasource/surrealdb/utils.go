package surrealdb

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/surrealdb/surrealdb.go/pkg/models"
	"go.opentelemetry.io/otel/trace"
)

var rgx = regexp.MustCompile(`\s+`)

var (
	errAlreadyExists    = errors.New("resource already exists")
	errUnexpectedFormat = errors.New("unexpected result format")
)

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

// processResults processes and extracts meaningful data from query results.
func (c *Client) processResults(query string, results *[]QueryResult) ([]any, error) {
	var resp []any

	if len(*results) > 0 {
		resp = make([]any, 0, len(*results))
	}

	for _, r := range *results {
		if err := c.processResult(query, r, &resp); err != nil {
			return nil, err
		}
	}

	return resp, nil
}

// processResult handles individual query result.
func (c *Client) processResult(query string, r QueryResult, resp *[]any) error {
	if r.Status != statusOK {
		return c.handleNonOKStatus(query, r, resp)
	}

	return c.handleOKStatus(r, resp)
}

// handleNonOKStatus handles non-OK status results.
func (c *Client) handleNonOKStatus(query string, r QueryResult, resp *[]any) error {
	if !isAdministrativeOperation(query) {
		c.logger.Errorf("query result error: %v", r.Result)
		return nil
	}

	handled, err := c.handleAdminError(r.Result, resp)
	if err != nil {
		return err
	}

	if handled {
		return nil
	}

	return c.processResultRecords(r.Result, resp)
}

// handleOKStatus handles OK status results.
func (c *Client) handleOKStatus(r QueryResult, resp *[]any) error {
	if isCustomNil(r.Result) {
		*resp = append(*resp, true)
		return nil
	}

	return c.processResultRecords(r.Result, resp)
}

// handleAdminError handles administrative operation errors.
func (*Client) handleAdminError(result any, resp *[]any) (bool, error) {
	if strErr, ok := result.(string); ok {
		if strings.Contains(strErr, "already exists") {
			return false, errAlreadyExists
		}

		return false, fmt.Errorf("%w: %s", errUnexpectedFormat, strErr)
	}

	if result == nil {
		*resp = append(*resp, true)
		return true, nil
	}

	return false, nil
}

// processResultRecords processes valid result records.
func (c *Client) processResultRecords(result any, resp *[]any) error {
	recordList, ok := result.([]any)
	if !ok {
		return errInvalidResult
	}

	for _, record := range recordList {
		extracted, err := c.extractRecord(record)
		if err != nil {
			return fmt.Errorf("failed to extract record: %w", err)
		}

		*resp = append(*resp, extracted)
	}

	return nil
}

// isCustomNil checks for CustomNil type.
func isCustomNil(result any) bool {
	_, ok := result.(models.CustomNil)
	return ok
}
