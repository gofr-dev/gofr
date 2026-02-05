package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	t.Setenv("APP_ENV", "dev")

	host := fmt.Sprintf("http://localhost:%d", httpPort)

	go main()
	time.Sleep(500 * time.Millisecond) // Wait for server and migrations

	t.Run("hello query", func(t *testing.T) {
		query := `{"query": "{ hello }"}`
		resp, err := http.Post(host+"/graphql", "application/json", bytes.NewBufferString(query))
		require.NoError(t, err)
		defer resp.Body.Close()

		var result struct {
			Data struct {
				Hello string `json:"hello"`
			} `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Equal(t, "Hello GoFr GraphQL with SQL!", result.Data.Hello)
	})

	t.Run("createUser mutation", func(t *testing.T) {
		query := `{"query": "mutation { createUser(name: \"Integration Test\", role: \"Tester\") { id name role } }"}`
		resp, err := http.Post(host+"/graphql", "application/json", bytes.NewBufferString(query))
		require.NoError(t, err)
		defer resp.Body.Close()

		var result struct {
			Data struct {
				CreateUser User `json:"createUser"`
			} `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		assert.True(t, result.Data.CreateUser.ID > 0)
		assert.Equal(t, "Integration Test", result.Data.CreateUser.Name)
	})

	t.Run("getUser query", func(t *testing.T) {
		// We expect ID 1 if it's the first user created
		query := `{"query": "{ getUser(id: 1) { id name role } }"}`
		resp, err := http.Post(host+"/graphql", "application/json", bytes.NewBufferString(query))
		require.NoError(t, err)
		defer resp.Body.Close()

		var result struct {
			Data struct {
				GetUser User `json:"getUser"`
			} `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Equal(t, 1, result.Data.GetUser.ID)
		assert.Equal(t, "Integration Test", result.Data.GetUser.Name)
	})

	t.Run("Playground UI presence", func(t *testing.T) {
		resp, err := http.Get(host + "/graphql/ui")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
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
