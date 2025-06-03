package pinecone

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// QueryLog represents a log entry for Pinecone operations.
type QueryLog struct {
	Operation   string         `json:"operation"`
	Duration    int64          `json:"duration"`
	Index       string         `json:"index,omitempty"`
	Namespace   string         `json:"namespace,omitempty"`
	VectorCount int            `json:"vectorCount,omitempty"`
	TopK        int            `json:"topK,omitempty"`
	Error       string         `json:"error,omitempty"`
	Filter      map[string]any `json:"filter,omitempty"`
	IDs         []string       `json:"ids,omitempty"`
}

// PrettyPrint formats and prints the QueryLog to the provided writer.
func (q *QueryLog) PrettyPrint(writer io.Writer) {
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")

	// Format the query details for display
	var details []string

	if q.Index != "" {
		details = append(details, fmt.Sprintf("index:%s", q.Index))
	}

	if q.Namespace != "" {
		details = append(details, fmt.Sprintf("namespace:%s", q.Namespace))
	}

	if q.VectorCount > 0 {
		details = append(details, fmt.Sprintf("vectors:%d", q.VectorCount))
	}

	if q.TopK > 0 {
		details = append(details, fmt.Sprintf("topK:%d", q.TopK))
	}

	if len(q.IDs) > 0 {
		// Limit the number of IDs shown in logs
		idDisplay := q.IDs
		if len(idDisplay) > 5 {
			idDisplay = idDisplay[:5]
		}
		details = append(details, fmt.Sprintf("ids:%s", strings.Join(idDisplay, ",")))
	}

	if q.Filter != nil {
		filterJSON, err := json.Marshal(q.Filter)
		if err == nil {
			// Truncate large filter strings
			filterStr := string(filterJSON)
			if len(filterStr) > 100 {
				filterStr = filterStr[:97] + "..."
			}
			details = append(details, fmt.Sprintf("filter:%s", filterStr))
		}
	}

	detailsStr := ""
	if len(details) > 0 {
		detailsStr = " " + strings.Join(details, " ")
	}

	// Format the duration
	var durationStr string
	switch {
	case q.Duration < 1000:
		durationStr = fmt.Sprintf("%dµs", q.Duration)
	case q.Duration < 1000000:
		durationStr = fmt.Sprintf("%.2fms", float64(q.Duration)/1000)
	default:
		durationStr = fmt.Sprintf("%.2fs", float64(q.Duration)/1000000)
	}

	// Add error information if present
	errorInfo := ""
	if q.Error != "" {
		errorInfo = fmt.Sprintf(" error:%s", q.Error)
	}

	// Write the formatted log entry to the writer
	fmt.Fprintf(writer, "\x1b[36m%s\x1b[0m PINECONE \x1b[32m%s\x1b[0m \x1b[33m[%s]\x1b[0m%s%s\n",
		timestamp, q.Operation, durationStr, detailsStr, errorInfo)
}
