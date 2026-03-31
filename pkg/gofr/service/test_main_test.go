package service

import (
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var errTest = errors.New("test error")

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func checkAuthHeaders(t *testing.T, r *http.Request) {
	t.Helper()

	authHeader := r.Header.Get(AuthHeader)

	if authHeader == "" {
		return
	}

	authParts := strings.Split(authHeader, " ")
	payload, _ := base64.StdEncoding.DecodeString(authParts[1])
	credentials := strings.Split(string(payload), ":")

	assert.Equal(t, "user", credentials[0])
	assert.Equal(t, "password", credentials[1])
}
