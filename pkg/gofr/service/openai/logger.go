package openai

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Logf(pattern string, args ...any)
	Errorf(pattern string, args ...any)
}

type APILog struct {
	ID                string `json:"id,omitempty"`
	Query             string `json:"query,omitempty"`
	Object            string `json:"object,omitempty"`
	Created           int    `json:"created,omitempty"`
	Model             string `json:"model,omitempty"`
	ServiceTier       string `json:"service_tier,omitempty"`
	SystemFingerprint string `json:"system_fingerprint,omitempty"`
	Duration          int64  `json:"duration,omitempty"`

	Usage struct {
		PromptTokens            int `json:"prompt_tokens,omitempty"`
		CompletionTokens        int `json:"completion_tokens,omitempty"`
		TotalTokens             int `json:"total_tokens,omitempty"`
		CompletionTokensDetails any `json:"completion_tokens_details,omitempty"`
		PromptTokensDetails     any `json:"prompt_tokens_details,omitempty"`
	} `json:"usage,omitempty"`

	Error *Error `json:"error,omitempty"`
}

func (al *APILog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer,
		"\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s\n",

		clean(al.Query),
		"OPENAI",
		al.Duration,
		clean(strings.Join([]string{al.Model, fmt.Sprint(al.Created), fmt.Sprint(al.Usage)}, " ")),
	)
}

func clean(query string) string {
	// Replace multiple consecutive whitespace characters with a single space
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")

	// Trim leading and trailing whitespace from the string
	query = strings.TrimSpace(query)

	return query
}
