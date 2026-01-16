package exporters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheusPush_Flush(t *testing.T) {
	// 1. Start a mock Pushgateway server
	var received bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and URL
		if (r.Method == http.MethodPost || r.Method == http.MethodPut) && r.URL.Path == "/metrics/job/test-job" {
			received = true

			w.WriteHeader(http.StatusAccepted)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	// 2. Initialize PrometheusPush with the mock server URL
	meter, flush := PrometheusPush("test-app", "v1.0.0", server.URL, "test-job", 0)

	assert.NotNil(t, meter)
	assert.NotNil(t, flush)

	// 3. Create a metric (to ensure registry isn't empty, though not strictly required for push to trigger)
	counter, err := meter.Int64Counter("test_counter")
	require.NoError(t, err)

	counter.Add(context.Background(), 1)

	// 4. Call flush
	err = flush(context.Background())

	// 5. Verify push was attempted
	require.NoError(t, err)
	assert.True(t, received, "Expected Pushgateway to receive a request")
}
