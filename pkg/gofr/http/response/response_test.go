package response

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResponse_SetCustomHeaders(t *testing.T) {
	tests := []struct {
		desc     string
		headers  map[string]string
		expected map[string]string
	}{
		{
			desc:     "nil headers map",
			headers:  nil,
			expected: map[string]string{},
		},
		{
			desc:     "empty headers map",
			headers:  map[string]string{},
			expected: map[string]string{},
		},
		{
			desc:     "single header",
			headers:  map[string]string{"X-Custom": "value"},
			expected: map[string]string{"X-Custom": "value"},
		},
		{
			desc: "multiple headers",
			headers: map[string]string{
				"X-Custom-One": "one",
				"X-Custom-Two": "two",
			},
			expected: map[string]string{
				"X-Custom-One": "one",
				"X-Custom-Two": "two",
			},
		},
	}

	for i, tc := range tests {
		w := httptest.NewRecorder()
		resp := Response{Headers: tc.headers}

		resp.SetCustomHeaders(w)

		for key, want := range tc.expected {
			assert.Equal(t, want, w.Header().Get(key), "TEST[%d], Failed.\n%s", i, tc.desc)
		}
	}
}

func TestResponse_SetCustomHeaders_OverwritesExisting(t *testing.T) {
	w := httptest.NewRecorder()
	w.Header().Set("X-Custom", "old")

	resp := Response{Headers: map[string]string{"X-Custom": "new"}}
	resp.SetCustomHeaders(w)

	assert.Equal(t, "new", w.Header().Get("X-Custom"))
}

func TestResponse_SetCustomHeaders_CanonicalizesKey(t *testing.T) {
	w := httptest.NewRecorder()

	resp := Response{Headers: map[string]string{"x-custom-header": "value"}}
	resp.SetCustomHeaders(w)

	// http.Header canonicalizes keys, so a lowercase key must be retrievable canonically.
	assert.Equal(t, "value", w.Header().Get("X-Custom-Header"))
}
