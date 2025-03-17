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

func TestListHandler(t *testing.T) {
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
