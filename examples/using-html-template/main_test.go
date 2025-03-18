package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func Test_ListHandler(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	c := &http.Client{}

	go main()
	time.Sleep(100 * time.Millisecond)

	// Make a GET request to the /list endpoint
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet,
		configs.HTTPHost+"/list", http.NoBody)
	resp, err := c.Do(req)

	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/html", resp.Header.Get("Content-Type"))

	// Read response body
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	bodyStr := strings.Join(strings.Fields(string(body)), " ")

	// Validate key HTML elements using strings.Contains
	assert.Contains(t, bodyStr, "<h2>My TODO list</h2>", "Header text missing")

	expectedItems := "<li>Expand on Gofr documentation </li> <li class=\"done\">" +
		"Add more examples</li> <li>Write some articles</li>"

	assert.Contains(t, bodyStr, expectedItems, "Missing TODO items")

	// Validate stylesheet link
	assert.Contains(t, bodyStr, `<link rel="stylesheet" href="style.css">`, "Stylesheet link missing")
}

func Test_IndexHTML(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	c := &http.Client{}

	go main()
	time.Sleep(100 * time.Millisecond) // Allow server to start

	// Request root endpoint
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet,
		configs.HTTPHost+"/", http.NoBody)
	resp, err := c.Do(req)

	require.NoError(t, err)
	defer resp.Body.Close()

	// Validate basic response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyStr := strings.Join(strings.Fields(string(body)), " ")

	// Validate index.html content
	assert.Contains(t, bodyStr, "<h1>Hello HTML!</h1>", "Main header missing")
	assert.Contains(t, bodyStr, `src="/favicon.ico"`, "Favicon reference missing")
	assert.Contains(t, bodyStr, `href="/list"`, "List endpoint link missing")
}

func Test_404HTML(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	c := &http.Client{}

	go main()
	time.Sleep(100 * time.Millisecond) // Allow server to start

	// Request non-existent endpoint
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet,
		configs.HTTPHost+"/non-existent-page", http.NoBody)
	resp, err := c.Do(req)

	require.NoError(t, err)
	defer resp.Body.Close()

	// Validate 404 response
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyStr := strings.Join(strings.Fields(string(body)), " ")

	// Validate 404.html content
	assert.Contains(t, bodyStr, "<h1>404 - Page Not Found</h1>", "404 header missing")
	assert.Contains(t, bodyStr, `href="index.html"`, "Home link missing")
	assert.Contains(t, bodyStr, "The requested resource could not be found", "Error message missing")
}
