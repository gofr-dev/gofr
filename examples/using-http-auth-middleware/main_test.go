package main

import (
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/testutil"
	"net/http"
	"testing"
	"time"
)

func Test_setupAPIKeyAuthFailed(t *testing.T) {
	serverConfigs := testutil.NewServerConfigs(t)

	// Run main() in a goroutine to avoid blocking
	go main()

	// Allow time for server to start
	time.Sleep(100 * time.Millisecond)

	client := &http.Client{Timeout: 200 * time.Millisecond}

	// Test invalid API key
	t.Run("Invalid API Key", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet,
			serverConfigs.HTTPHost+"/test-auth", http.NoBody)
		req.Header.Set("X-Api-Key", "test-key")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Error making request: %v", err)
		}
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

func Test_setupAPIKeyAuthSuccess(t *testing.T) {
	serverConfigs := testutil.NewServerConfigs(t)

	// Run main() in a goroutine to avoid blocking
	go main()

	// Allow time for server to start
	time.Sleep(100 * time.Millisecond)

	client := &http.Client{Timeout: 200 * time.Millisecond}

	// Test valid API key
	t.Run("Valid API Key", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet,
			serverConfigs.HTTPHost+"/test-auth", http.NoBody)
		req.Header.Set("X-Api-Key", "valid-api-key")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Error making request: %v", err)
		}
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

}

//func encodeBasicAuthorization(t *testing.T, arg string) string {
//	t.Helper()
//
//	data := []byte(arg)
//
//	dst := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
//
//	base64.StdEncoding.Encode(dst, data)
//
//	s := "Basic " + string(dst)
//
//	return s
//}

//func Test_setupBasicAuthSuccess(t *testing.T) {
//	serverConfigs := testutil.NewServerConfigs(t)
//
//	app := gofr.New()
//
//	setupBasicAuth(app)
//
//	app.GET("/basic-auth-success", func(_ *gofr.Context) (any, error) {
//		return "success", nil
//	})
//
//	go app.Run()
//
//	time.Sleep(100 * time.Millisecond)
//
//	var netClient = &http.Client{
//		Timeout: 200 * time.Millisecond,
//	}
//
//	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet,
//		serverConfigs.HTTPHost + "/basic-auth-success", http.NoBody)
//
//	req.Header.Add("Authorization", encodeBasicAuthorization(t, "username:password"))
//
//	// Send the request and check for successful response
//	resp, err := netClient.Do(req)
//	if err != nil {
//		t.Errorf("error while making HTTP request in Test_BasicAuthMiddleware. err: %v", err)
//		return
//	}
//
//	defer resp.Body.Close()
//
//	assert.Equal(t, http.StatusOK, resp.StatusCode, "Test_setupBasicAuthSuccess")
//}

//func Test_setupBasicAuthFailed(t *testing.T) {
//	serverConfigs := testutil.NewServerConfigs(t)
//
//	app := gofr.New()
//
//	setupBasicAuth(app)
//
//	app.GET("/basic-auth-failure", func(_ *gofr.Context) (any, error) {
//		return "success", nil
//	})
//
//	go app.Run()
//
//	time.Sleep(100 * time.Millisecond)
//
//	var netClient = &http.Client{
//		Timeout: 200 * time.Millisecond,
//	}
//
//	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet,
//		serverConfigs.HTTPHost + "/basic-auth-failure", http.NoBody)
//
//	req.Header.Add("Authorization", encodeBasicAuthorization(t, "username"))
//
//	// Send the request and check for successful response
//	resp, err := netClient.Do(req)
//	if err != nil {
//		t.Errorf("error while making HTTP request in Test_BasicAuthMiddleware. err: %v", err)
//		return
//	}
//
//	defer resp.Body.Close()
//
//	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Test_setupBasicAuthFailed")
//}
