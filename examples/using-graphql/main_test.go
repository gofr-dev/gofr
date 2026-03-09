package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/testutil"
)

func waitForReady(t *testing.T, host string) {
	t.Helper()
	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(10 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := client.Get(host + "/.well-known/alive")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("Server at %s not ready after 10s", host)
}

// newTestApp creates a GoFr application configured for integration testing.
func newTestApp(t *testing.T) (*gofr.App, string) {
	t.Helper()

	httpPort := testutil.GetFreePort(t)
	metricsPort := testutil.GetFreePort(t)

	t.Setenv("HTTP_PORT", strconv.Itoa(httpPort))
	t.Setenv("METRICS_PORT", strconv.Itoa(metricsPort))

	host := fmt.Sprintf("http://localhost:%d", httpPort)

	app := gofr.New()

	app.GraphQLQuery("hello", func(c *gofr.Context) (interface{}, error) {
		return "Hello GoFr GraphQL with SQL!", nil
	})

	app.GraphQLQuery("getUser", func(c *gofr.Context) (interface{}, error) {
		var args struct {
			ID int `json:"id"`
		}

		if err := c.Bind(&args); err != nil {
			return nil, err
		}

		// Return stubbed data instead of calling c.SQL
		return User{ID: args.ID, Name: "Test User", Role: "Admin"}, nil
	})

	app.GraphQLMutation("createUser", func(c *gofr.Context) (interface{}, error) {
		var args struct {
			Name string `json:"name"`
			Role string `json:"role"`
		}

		if err := c.Bind(&args); err != nil {
			return nil, err
		}

		// Return stubbed data instead of calling c.SQL
		return User{ID: 1, Name: args.Name, Role: args.Role}, nil
	})

	return app, host
}

func TestIntegration_GraphQL(t *testing.T) {
	t.Setenv("APP_ENV", "dev")

	app, host := newTestApp(t)
	go app.Run()

	waitForReady(t, host)

	defer app.Shutdown(context.Background())

	t.Run("hello query", func(t *testing.T) {
		query := `{"query": "{ hello }"}`
		resp, err := http.Post(host+"/graphql", "application/json", bytes.NewBufferString(query))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

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

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result struct {
			Data struct {
				CreateUser User `json:"createUser"`
			} `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Greater(t, result.Data.CreateUser.ID, 0)
		assert.Equal(t, "Integration Test", result.Data.CreateUser.Name)
		assert.Equal(t, "Tester", result.Data.CreateUser.Role)
	})

	t.Run("getUser query", func(t *testing.T) {
		query := `{"query": "{ getUser(id: 1) { id name role } }"}`
		resp, err := http.Post(host+"/graphql", "application/json", bytes.NewBufferString(query))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result struct {
			Data struct {
				GetUser User `json:"getUser"`
			} `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Equal(t, 1, result.Data.GetUser.ID)
		assert.NotEmpty(t, result.Data.GetUser.Name)
	})

	t.Run("playground UI is accessible", func(t *testing.T) {
		resp, err := http.Get(host + "/.well-known/graphql/ui")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
