package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isWellKnown(t *testing.T) {
	tests := []struct {
		desc     string
		endpoint string
		resp     bool
	}{
		{"empty endpoint", "", false},
		{"sample endpoint", "/sample", false},
		{"health-check endpoint", "/.well-known/health-check", true},
		{"alive endpoint", "/.well-known/alive", true},
	}

	for i, tc := range tests {
		resp := isWellKnown(tc.endpoint)

		assert.Equal(t, tc.resp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
