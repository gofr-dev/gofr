package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/internal/errhide"
)

// statusError is an app error that declares a status code, so its text was written for clients.
type statusError struct{ msg string }

func (e statusError) Error() string { return e.msg }
func (statusError) StatusCode() int { return http.StatusInternalServerError }

// hideErrors turns HIDE_INTERNAL_ERRORS on or off for one test and restores the previous value.
// The switch is process-wide, so tests that use it must not run in parallel.
func hideErrors(t *testing.T, on bool) {
	t.Helper()

	prev := errhide.Enabled()

	errhide.Enable(on)
	t.Cleanup(func() { errhide.Enable(prev) })
}

func callToolText(t *testing.T, s *Server) toolResult {
	t.Helper()

	resp := decode(t, post(t, s, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_user","arguments":{}}}`))
	require.Nil(t, resp.Error)

	var res toolResult

	raw, err := json.Marshal(resp.Result)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &res))
	require.Len(t, res.Content, 1)

	return res
}

func TestServer_HideInternalErrors_ToolError(t *testing.T) {
	tests := []struct {
		desc     string
		on       bool
		err      error
		wantText string
		logged   bool
	}{
		{"on: raw tool error is hidden", true, errDBUnavailable, "Internal Server Error", true},
		{"on: wrapped tool error is hidden", true, fmt.Errorf("get_user: %w", errDBUnavailable), "Internal Server Error", true},
		{"on: error with a status code is kept", true, statusError{"payment provider is down, retry later"},
			"payment provider is down, retry later", false},
		{"off: raw tool error is unchanged", false, errDBUnavailable, "db unavailable", false},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			hideErrors(t, tc.on)

			logger := &fakeLogger{}
			tools := &fakeTools{
				specs: []ai.ToolSpec{readTool()},
				call: func(context.Context, string, json.RawMessage) (ai.Result, error) {
					return ai.Result{}, tc.err
				},
			}

			res := callToolText(t, NewServer(tools, WithLogger(logger)))

			assert.True(t, res.IsError)
			assert.Equal(t, tc.wantText, res.Content[0].Text)

			if tc.logged {
				assert.Contains(t, strings.Join(logger.all(), "\n"), tc.err.Error(), "the original error must be logged")
			}
		})
	}
}

func TestServer_HideInternalErrors_PanicKeepsGoFrText(t *testing.T) {
	hideErrors(t, true)

	tools := &fakeTools{
		specs: []ai.ToolSpec{readTool()},
		call:  func(context.Context, string, json.RawMessage) (ai.Result, error) { panic("boom") },
	}

	res := callToolText(t, NewServer(tools))

	assert.Equal(t, errToolPanicked.Error(), res.Content[0].Text)
}

func TestServer_HideInternalErrors_HookError(t *testing.T) {
	tests := []struct {
		desc        string
		on          bool
		err         error
		wantMessage string
	}{
		{"on: raw hook error is hidden", true, errAccessDenied, "Internal Server Error"},
		{"on: hook error with a status code is kept", true, statusError{"tool disabled for this tenant"},
			"tool disabled for this tenant"},
		{"off: raw hook error is unchanged", false, errAccessDenied, "access denied"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			hideErrors(t, tc.on)

			logger := &fakeLogger{}
			s := NewServer(&fakeTools{specs: []ai.ToolSpec{readTool()}}, WithLogger(logger),
				WithHook(func(context.Context, ai.ToolSpec) error { return tc.err }))

			resp := decode(t, post(t, s, `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"get_user","arguments":{}}}`))

			require.NotNil(t, resp.Error)
			assert.Equal(t, codeInternal, resp.Error.Code)
			assert.Equal(t, tc.wantMessage, resp.Error.Message)

			if tc.on && tc.wantMessage == errhide.Message {
				assert.Contains(t, strings.Join(logger.all(), "\n"), tc.err.Error(), "the original error must be logged")
			}
		})
	}
}
