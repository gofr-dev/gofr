package service

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

func TestBasicAuthProvider_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	path := "/path"
	queryParams := map[string]interface{}{"key": "value"}
	body := []byte("body")

	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkAuthHeaders(t, r)
		assert.Equal(t, http.MethodGet, r.Method)

		w.WriteHeader(http.StatusOK)

		_, err := w.Write(body)
		if err != nil {
			return
		}
	}))
	defer server.Close()

	// Create a new HTTP service instance with basic auth
	httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&BasicAuthConfig{UserName: "user", Password: "cGFzc3dvcmQ="})

	// Make the GET request
	resp, err := httpService.Get(context.Background(), path, queryParams)
	require.NoError(t, err)

	defer resp.Body.Close()

	// Check response status code and body (if applicable)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, err)

	bodyBytes, _ := io.ReadAll(resp.Body)

	assert.Equal(t, string(body), string(bodyBytes))
}

func TestBasicAuthProvider_Post(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	path := "/path"
	queryParams := map[string]interface{}{"key": "value"}
	body := []byte("body")

	// Create a mock HTTP server (verify POST method)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkAuthHeaders(t, r)
		assert.Equal(t, http.MethodPost, r.Method)

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	// Create a new HTTP service instance with basic auth
	httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&BasicAuthConfig{UserName: "user", Password: "cGFzc3dvcmQ="})

	// Make the POST request
	resp, err := httpService.Post(context.Background(), path, queryParams, body)
	require.NoError(t, err)

	defer resp.Body.Close()

	// Check response status code (no body assertion for POST)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NoError(t, err)
}

func TestBasicAuthProvider_Put(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	path := "/path"
	queryParams := map[string]interface{}{"key": "value"}
	body := []byte("body")

	// Create a mock HTTP server (verify PUT method)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkAuthHeaders(t, r)
		assert.Equal(t, http.MethodPut, r.Method)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a new HTTP service instance with basic auth
	httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&BasicAuthConfig{UserName: "user", Password: "cGFzc3dvcmQ="})

	// Make the PUT request
	resp, err := httpService.Put(context.Background(), path, queryParams, body)
	require.NoError(t, err)

	defer resp.Body.Close()

	// Check response status code
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, err)
}

func TestBasicAuthProvider_Patch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	path := "/path"
	queryParams := map[string]interface{}{"key": "value"}
	body := []byte("body")

	// Create a mock HTTP server (verify PATCH method)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkAuthHeaders(t, r)
		assert.Equal(t, http.MethodPatch, r.Method)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a new HTTP service instance with basic auth
	httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&BasicAuthConfig{UserName: "user", Password: "cGFzc3dvcmQ="})

	// Make the PATCH request
	resp, err := httpService.Patch(context.Background(), path, queryParams, body)
	require.NoError(t, err)

	defer resp.Body.Close()

	// Check response status code
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, err)
}

func TestBasicAuthProvider_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	path := "/path"
	body := []byte("body")

	// Create a mock HTTP server (verify DELETE method)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkAuthHeaders(t, r)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	// Create a new HTTP service instance with basic auth
	httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&BasicAuthConfig{UserName: "user", Password: "cGFzc3dvcmQ="})

	// Make the DELETE request
	resp, err := httpService.Delete(context.Background(), path, body)
	require.NoError(t, err)

	defer resp.Body.Close()

	// Check response status code (no body assertion for DELETE)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.NoError(t, err)
}

func checkAuthHeaders(t *testing.T, r *http.Request) {
	t.Helper()

	authHeader := r.Header.Get("Authorization")

	if authHeader == "" {
		return
	}

	authParts := strings.Split(authHeader, " ")
	payload, _ := base64.StdEncoding.DecodeString(authParts[1])
	credentials := strings.Split(string(payload), ":")

	assert.Equal(t, "user", credentials[0])
	assert.Equal(t, "password", credentials[1])
}

func Test_addAuthorizationHeader_Error(t *testing.T) {
	ba := &basicAuthProvider{password: "invalid_password"}

	headers := make(map[string]string)
	err := ba.addAuthorizationHeader(headers)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	expectedErrMsg := "illegal base64 data at input byte 7"
	require.ErrorContains(t, err, expectedErrMsg, "Test_addAuthorizationHeader_Error Failed!")
}
