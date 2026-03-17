package exporters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLogger struct {
	logs   []string
	errors []string
}

func (m *mockLogger) Logf(_ string, _ ...any) {
	m.logs = append(m.logs, "log")
}

func (m *mockLogger) Errorf(_ string, _ ...any) {
	m.errors = append(m.errors, "error")
}

func TestNewPushGateway(t *testing.T) {
	l := &mockLogger{}
	pg := NewPushGateway("http://localhost:9091", "test-job", l)

	assert.NotNil(t, pg)
	assert.NotNil(t, pg.pusher)
	assert.Equal(t, l, pg.logger)
}

func TestPushGateway_Push_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	l := &mockLogger{}
	pg := NewPushGateway(server.URL, "test-job", l)

	err := pg.Push(context.Background())

	require.NoError(t, err)
	assert.NotEmpty(t, l.logs)
}

func TestPushGateway_Push_Failure(t *testing.T) {
	l := &mockLogger{}
	pg := NewPushGateway("http://localhost:1", "test-job", l)

	err := pg.Push(context.Background())

	require.Error(t, err)
	assert.NotEmpty(t, l.errors)
}
