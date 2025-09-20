package main

import (
	"encoding/base64"
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/testutil"
	"net/http"
	"testing"
	"time"
)

func Test_main(t *testing.T) {
	go main()
}

func Test_setupBasicAuthSuccess(t *testing.T) {
	serverConfigs := testutil.NewServerConfigs(t)

	app := gofr.New()

	setupBasicAuth(app)

	app.GET("/basic-auth-success", func(_ *gofr.Context) (any, error) {
		return "success", nil
	})

	go app.Run()

	time.Sleep(100 * time.Millisecond)

	var netClient = &http.Client{
		Timeout: 200 * time.Millisecond,
	}

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet,
		serverConfigs.HTTPHost + "/basic-auth-success", http.NoBody)

	req.Header.Add("Authorization", encodeBasicAuthorization(t, "username:password"))

	// Send the request and check for successful response
	resp, err := netClient.Do(req)
	if err != nil {
		t.Errorf("error while making HTTP request in Test_BasicAuthMiddleware. err: %v", err)
		return
	}

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Test_setupBasicAuthSuccess")
}

func Test_setupBasicAuthFailed(t *testing.T) {
	serverConfigs := testutil.NewServerConfigs(t)

	app := gofr.New()

	setupBasicAuth(app)

	app.GET("/basic-auth-failure", func(_ *gofr.Context) (any, error) {
		return "success", nil
	})

	go app.Run()

	time.Sleep(100 * time.Millisecond)

	var netClient = &http.Client{
		Timeout: 200 * time.Millisecond,
	}

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet,
		serverConfigs.HTTPHost + "/basic-auth-failure", http.NoBody)

	req.Header.Add("Authorization", encodeBasicAuthorization(t, "username"))

	// Send the request and check for successful response
	resp, err := netClient.Do(req)
	if err != nil {
		t.Errorf("error while making HTTP request in Test_BasicAuthMiddleware. err: %v", err)
		return
	}

	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Test_setupBasicAuthFailed")
}

func Test_setupAPIKeyAuthFailed(t *testing.T) {
	serverConfigs := testutil.NewServerConfigs(t)

	app := gofr.New()

	setupAPIKeyAuth(app)

	app.GET("/api-key-failure", func(_ *gofr.Context) (any, error) {
		return "success", nil
	})

	go app.Run()

	time.Sleep(100 * time.Millisecond)

	var netClient = &http.Client{
		Timeout: 200 * time.Millisecond,
	}

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet,
		serverConfigs.HTTPHost +"/api-key-failure", http.NoBody)

	req.Header.Set("X-Api-Key", "test-key")

	// Send the request and check for successful response
	resp, err := netClient.Do(req)
	if err != nil {
		t.Errorf("error while making HTTP request in Test_APIKeyAuthMiddleware. err: %v", err)
		return
	}

	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Test_setupAPIKeyAuthFailed")
}

func Test_setupAPIKeyAuthSuccess(t *testing.T) {
	serverConfigs := testutil.NewServerConfigs(t)

	app := gofr.New()

	setupAPIKeyAuth(app)

	app.GET("/api-key-success", func(_ *gofr.Context) (any, error) {
		return "success", nil
	})

	go app.Run()

	time.Sleep(100 * time.Millisecond)

	var netClient = &http.Client{
		Timeout: 200 * time.Millisecond,
	}

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet,
		serverConfigs.HTTPHost + "/api-key-success", http.NoBody)
	req.Header.Set("X-Api-Key", "valid-api-key")

	// Send the request and check for successful response
	resp, err := netClient.Do(req)
	if err != nil {
		t.Errorf("error while making HTTP request in Test_APIKeyAuthMiddleware. err: %v", err)
		return
	}

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Test_setupAPIKeyAuthSuccess")
}

func encodeBasicAuthorization(t *testing.T, arg string) string {
	t.Helper()

	data := []byte(arg)

	dst := make([]byte, base64.StdEncoding.EncodedLen(len(data)))

	base64.StdEncoding.Encode(dst, data)

	s := "Basic " + string(dst)

	return s
}