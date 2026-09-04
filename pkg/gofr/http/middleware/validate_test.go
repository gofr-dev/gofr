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
		{"well-known root", "/.well-known", true},
		{"prefixed endpoint without separator", "/.well-knownprivate", false},
		{"prefixed endpoint with sub path", "/.well-knownprivate/secret", false},
		{"well-known not at the start of the path", "/api/.well-known/alive", false},
	}

	for i, tc := range tests {
		resp := isWellKnown(tc.endpoint)

		assert.Equal(t, tc.resp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
