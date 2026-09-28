package http

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseCapture(t *testing.T) {
	tests := []struct {
		desc      string
		maxBytes  int
		write     func(c *ResponseCapture) []int
		expStatus int
		expBody   string
		expN      []int
	}{
		{
			desc:      "defaults to 200 with no writes",
			maxBytes:  10,
			write:     func(*ResponseCapture) []int { return nil },
			expStatus: http.StatusOK,
			expBody:   "",
		},
		{
			desc:     "first WriteHeader wins",
			maxBytes: 10,
			write: func(c *ResponseCapture) []int {
				c.WriteHeader(http.StatusCreated)
				c.WriteHeader(http.StatusBadRequest)

				return nil
			},
			expStatus: http.StatusCreated,
			expBody:   "",
		},
		{
			desc:     "write before WriteHeader locks status to 200",
			maxBytes: 10,
			write: func(c *ResponseCapture) []int {
				n := writeAll(t, c, "hi")
				c.WriteHeader(http.StatusTeapot)

				return n
			},
			expStatus: http.StatusOK,
			expBody:   "hi",
			expN:      []int{2},
		},
		{
			desc:      "body within cap",
			maxBytes:  10,
			write:     func(c *ResponseCapture) []int { return writeAll(t, c, "hello", "gofr") },
			expStatus: http.StatusOK,
			expBody:   "hellogofr",
			expN:      []int{5, 4},
		},
		{
			desc:      "body truncated at cap and further writes dropped",
			maxBytes:  7,
			write:     func(c *ResponseCapture) []int { return writeAll(t, c, "hello", "world", "!") },
			expStatus: http.StatusOK,
			expBody:   "hellowo",
			expN:      []int{5, 5, 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			c := NewResponseCapture(tc.maxBytes)

			n := tc.write(c)

			assert.Equal(t, tc.expN, n)
			assert.Equal(t, tc.expStatus, c.Status())
			assert.Equal(t, tc.expBody, string(c.Body()))
		})
	}
}

func TestResponseCapture_Header(t *testing.T) {
	c := NewResponseCapture(1)

	c.Header().Set("Content-Type", "application/json")

	assert.Equal(t, "application/json", c.Header().Get("Content-Type"))
}

func writeAll(t *testing.T, c *ResponseCapture, chunks ...string) []int {
	t.Helper()

	written := make([]int, 0, len(chunks))

	for _, chunk := range chunks {
		n, err := c.Write([]byte(chunk))
		require.NoError(t, err)

		written = append(written, n)
	}

	return written
}
