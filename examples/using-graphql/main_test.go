package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

func TestIntegration_GraphQL(t *testing.T) {
	httpPort := testutil.GetFreePort(t)
	metricsPort := testutil.GetFreePort(t)

	t.Setenv("HTTP_PORT", strconv.Itoa(httpPort))
	t.Setenv("METRICS_PORT", strconv.Itoa(metricsPort))
	t.Setenv("APP_ENV", "dev") // Ensure UI is registered

	host := fmt.Sprintf("http://localhost:%d", httpPort)

	go main()
	time.Sleep(200 * time.Millisecond) // Wait for server to start

	tests := []struct {
		desc       string
		query      string
		expectData any
	}{
		{
			desc:  "hello query",
			query: `{"query": "{ hello }"}`,
			expectData: map[string]any{
				"hello": "Hello GoFr GraphQL with Reflection!",
			},
		},
		{
			desc:  "user query",
			query: `{"query": "{ user { id name role } }"}`,
			expectData: map[string]any{
				"user": map[string]any{
					"id":   float64(1),
					"name": "GoFr Developer",
					"role": "Maintainer",
				},
			},
		},
		{
			desc:  "users list query",
			query: `{"query": "{ users { id name } }"}`,
			expectData: map[string]any{
				"users": []any{
					map[string]any{"id": float64(1), "name": "Alice"},
					map[string]any{"id": float64(2), "name": "Bob"},
				},
			},
		},
		{
			desc:  "getUser query with arguments",
			query: `{"query": "{ getUser(id: 2) { id name } }"}`,
			expectData: map[string]any{
				"getUser": map[string]any{
					"id":   float64(2),
					"name": "Bob",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			resp, err := http.Post(host+"/graphql", "application/json", bytes.NewBufferString(tc.query))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var result struct {
				Data any `json:"data"`
			}

			body, _ := io.ReadAll(resp.Body)
			err = json.Unmarshal(body, &result)
			require.NoError(t, err)

			assert.Equal(t, tc.expectData, result.Data)
		})
	}

	t.Run("Playground UI presence", func(t *testing.T) {
		resp, err := http.Get(host + "/graphql/ui")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		assert.Contains(t, string(body), "GoFr GraphQL Playground")
	})
}

func TestIntegration_GraphQL_Production(t *testing.T) {
	// This test will verify that in production environment, the UI is NOT registered.
	// We run a separate test case that starts the app with APP_ENV=production.
	
	httpPort := testutil.GetFreePort(t)
	metricsPort := testutil.GetFreePort(t)

	t.Setenv("HTTP_PORT", strconv.Itoa(httpPort))
	t.Setenv("METRICS_PORT", strconv.Itoa(metricsPort))
	t.Setenv("APP_ENV", "production") 

	host := fmt.Sprintf("http://localhost:%d", httpPort)

	go main()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(host + "/graphql/ui")
	require.NoError(t, err)
	defer resp.Body.Close()

	// In GoFr, unregistered routes return 404 with a specific JSON body via catchAllHandler
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
