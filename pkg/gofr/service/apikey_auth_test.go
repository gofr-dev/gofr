package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

func Test_APIKeyAuthProvider_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	path := "/path"
	queryParams := map[string]interface{}{"key": "value"}
	body := []byte("body")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)

		w.WriteHeader(http.StatusOK)

		_, err := w.Write(body)
		if err != nil {
			return
		}
	}))
	defer server.Close()

	httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&APIKeyConfig{"valid-key"})

	resp, err := httpService.Get(context.Background(), path, queryParams)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, err)

	bodyBytes, _ := io.ReadAll(resp.Body)

	assert.Equal(t, string(body), string(bodyBytes))
}

func Test_APIKeyAuthProvider_Post(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	path := "/path"
	queryParams := map[string]interface{}{"key": "value"}
	body := []byte("body")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&APIKeyConfig{"valid-key"})

	resp, err := httpService.Post(context.Background(), path, queryParams, body)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NoError(t, err)
}

func TestApiKeyProvider_Put(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	path := "/path"
	queryParams := map[string]interface{}{"key": "value"}
	body := []byte("body")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&APIKeyConfig{"valid-key"})

	resp, err := httpService.Put(context.Background(), path, queryParams, body)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, err)
}

func TestApiKeyAuthProvider_Patch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	path := "/path"
	queryParams := map[string]interface{}{"key": "value"}
	body := []byte("body")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&APIKeyConfig{"valid-key"})

	resp, err := httpService.Patch(context.Background(), path, queryParams, body)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, err)
}

func TestApiKeyAuthProvider_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	path := "/path"
	body := []byte("body")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&APIKeyConfig{"valid-key"})

	resp, err := httpService.Delete(context.Background(), path, body)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.NoError(t, err)
}
