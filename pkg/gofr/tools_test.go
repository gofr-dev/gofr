package gofr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isFrameworkRoute(t *testing.T) {
	tests := []struct {
		desc         string
		pathTemplate string
		resp         bool
	}{
		{"empty path", "", false},
		{"application route", "/sample", false},
		{"favicon", "/favicon.ico", true},
		{"well-known root", "/.well-known", true},
		{"alive endpoint", "/.well-known/alive", true},
		{"health endpoint", "/.well-known/health", true},
		{"swagger endpoint", "/.well-known/swagger", true},
		{"prefixed route without separator", "/.well-knownstuff", false},
		{"prefixed route with sub path", "/.well-knownstuff/data", false},
		{"well-known not at the start of the path", "/api/.well-known/alive", false},
	}

	for i, tc := range tests {
		resp := isFrameworkRoute(tc.pathTemplate)

		assert.Equal(t, tc.resp, resp, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
