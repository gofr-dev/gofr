package surrealdb

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type Logger interface {
	Debugf(pattern string, args ...interface{})
	Debug(args ...interface{})
	Logf(pattern string, args ...interface{})
	Errorf(pattern string, args ...interface{})
}

type Metrics interface {
	NewCounter(name, desc string)
	NewUpDownCounter(name, desc string)
	NewHistogram(name, desc string, buckets ...float64)
	NewGauge(name, desc string)

	IncrementCounter(ctx context.Context, name string, labels ...string)
	DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
	SetGauge(name string, value float64, labels ...string)
}

type QueryLog struct {
	Query      string      `json:"query"`
	Duration   int64       `json:"duration"`
	Namespace  string      `json:"namespace"`
	Database   string      `json:"database"`
	ID         interface{} `json:"id"`
	Data       interface{} `json:"data"`
	Filter     interface{} `json:"filter,omitempty"`
	Update     interface{} `json:"update,omitempty"`
	Collection string      `json:"collection,omitempty"`
}

func (ql *QueryLog) PrettyPrint(writer io.Writer) {
	if ql.Filter == nil {
		ql.Filter = ""
	}

	if ql.ID == nil {
		ql.ID = ""
	}

	if ql.Update == nil {
		ql.Update = ""
	}

	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s\n",
		clean(ql.Query), "SURREAL", ql.Duration,
		clean(strings.Join([]string{ql.Collection, fmt.Sprint(ql.Filter), fmt.Sprint(ql.ID), fmt.Sprint(ql.Update)}, " ")))
}

// clean takes a string query as input and performs two operations to clean it up:
// 1. It replaces multiple consecutive whitespace characters with a single space.
// 2. It trims leading and trailing whitespace from the string.
// The cleaned-up query string is then returned.
func clean(query string) string {
	// Replace multiple consecutive whitespace characters with a single space
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")

	// Trim leading and trailing whitespace from the string
	query = strings.TrimSpace(query)

	return query
}
