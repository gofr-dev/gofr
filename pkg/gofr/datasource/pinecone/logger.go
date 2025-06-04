package pinecone

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	// Display limits for logging
	maxFilterDisplayLength = 100
	maxIDsToDisplay       = 5
	filterTruncationSuffix = "..."

	// Time formatting constants
	microsecondsPerSecond = 1_000_000
	millisecondsPerSecond = 1_000

	// Color codes for console output
	colorTimestamp = "\x1b[36m"
	colorOperation = "\x1b[32m"
	colorDuration  = "\x1b[33m"
	colorError     = "\x1b[31m"
	colorReset     = "\x1b[0m"
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
	detailsStr := q.buildDetailsString()
	durationStr := q.formatDuration()
	errorInfo := q.formatError()

	fmt.Fprintf(writer, "%s%s%s PINECONE %s%s%s %s[%s]%s%s%s\n",
		colorTimestamp, timestamp, colorReset,
		colorOperation, q.Operation, colorReset,
		colorDuration, durationStr, colorReset,
		detailsStr, errorInfo)
}

// buildDetailsString constructs the details portion of the log entry.
func (q *QueryLog) buildDetailsString() string {
	var details []string

	details = q.addIndexDetails(details)
	details = q.addVectorDetails(details)
	details = q.addFilterDetails(details)

	if len(details) > 0 {
		return " " + strings.Join(details, " ")
	}
	return ""
}

// addIndexDetails adds index and namespace information to details.
func (q *QueryLog) addIndexDetails(details []string) []string {
	if q.Index != "" {
		details = append(details, fmt.Sprintf("index:%s", q.Index))
	}
	if q.Namespace != "" {
		details = append(details, fmt.Sprintf("namespace:%s", q.Namespace))
	}
	return details
}

// addVectorDetails adds vector-related information to details.
func (q *QueryLog) addVectorDetails(details []string) []string {
	if q.VectorCount > 0 {
		details = append(details, fmt.Sprintf("vectors:%d", q.VectorCount))
	}
	if q.TopK > 0 {
		details = append(details, fmt.Sprintf("topK:%d", q.TopK))
	}
	if len(q.IDs) > 0 {
		details = append(details, q.formatIDs())
	}
	return details
}

// addFilterDetails adds filter information to details.
func (q *QueryLog) addFilterDetails(details []string) []string {
	if q.Filter != nil {
		if filterStr := q.formatFilter(); filterStr != "" {
			details = append(details, fmt.Sprintf("filter:%s", filterStr))
		}
	}
	return details
}

// formatIDs formats the IDs for display, limiting to first 5.
func (q *QueryLog) formatIDs() string {
	idsToShow := q.getIDsForDisplay()
	return fmt.Sprintf("ids:%s", strings.Join(idsToShow, ","))
}

// getIDsForDisplay returns a slice of IDs limited for display purposes.
func (q *QueryLog) getIDsForDisplay() []string {
	if len(q.IDs) <= maxIDsToDisplay {
		return q.IDs
	}
	return q.IDs[:maxIDsToDisplay]
}

// formatFilter formats the filter for display, truncating if too long.
func (q *QueryLog) formatFilter() string {
	filterJSON, err := json.Marshal(q.Filter)
	if err != nil {
		return ""
	}

	return q.truncateFilterString(string(filterJSON))
}

// truncateFilterString truncates filter string if it exceeds maximum length.
func (q *QueryLog) truncateFilterString(filterStr string) string {
	if len(filterStr) <= maxFilterDisplayLength {
		return filterStr
	}

	truncateIndex := maxFilterDisplayLength - len(filterTruncationSuffix)
	return filterStr[:truncateIndex] + filterTruncationSuffix
}

// formatDuration formats the duration for display.
func (q *QueryLog) formatDuration() string {
	if q.Duration >= microsecondsPerSecond {
		seconds := float64(q.Duration) / microsecondsPerSecond
		return fmt.Sprintf("%.2fs", seconds)
	}

	if q.Duration >= millisecondsPerSecond {
		milliseconds := float64(q.Duration) / millisecondsPerSecond
		return fmt.Sprintf("%.2fms", milliseconds)
	}

	return fmt.Sprintf("%dµs", q.Duration)
}

// formatError formats the error information for display.
func (q *QueryLog) formatError() string {
	if q.Error != "" {
		return fmt.Sprintf(" %serror:%s%s", colorError, q.Error, colorReset)
	}
	return ""
}
