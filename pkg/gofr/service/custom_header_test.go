package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

func Test_CustomDomainProvider_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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

	customHeaderService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&DefaultHeaders{
			Headers: map[string]string{
				"TEST_KEY": "test_value",
			},
		})

	resp, err := customHeaderService.Get(context.Background(), "/path", queryParams)
	assert.Nil(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Nil(t, err)

	bodyBytes, _ := io.ReadAll(resp.Body)

	assert.Equal(t, string(body), string(bodyBytes))
}

func Test_CustomDomainProvider_Post(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	queryParams := map[string]interface{}{"key": "value"}
	body := []byte("body")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	customHeaderService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&DefaultHeaders{
			Headers: map[string]string{
				"TEST_KEY": "test_value",
			}})

	resp, err := customHeaderService.Post(context.Background(), "/path", queryParams, body)
	assert.Nil(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Nil(t, err)
}

func TestCustomDomainProvider_Put(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	queryParams := map[string]interface{}{"key": "value"}
	body := []byte("body")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	customHeaderService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&DefaultHeaders{
			Headers: map[string]string{
				"TEST_KEY": "test_value",
			}})

	resp, err := customHeaderService.Put(context.Background(), "/path", queryParams, body)
	assert.Nil(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Nil(t, err)
}

func TestCustomDomainProvider_Patch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	queryParams := map[string]interface{}{"key": "value"}
	body := []byte("body")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	customHeaderService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&DefaultHeaders{
			Headers: map[string]string{
				"TEST_KEY": "test_value",
			}})

	resp, err := customHeaderService.Patch(context.Background(), "/path", queryParams, body)
	assert.Nil(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Nil(t, err)
}

func TestCustomDomainProvider_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	body := []byte("body")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	customHeaderService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&DefaultHeaders{
			Headers: map[string]string{
				"TEST_KEY": "test_value",
			}})

	resp, err := customHeaderService.Delete(context.Background(), "/path", body)
	assert.Nil(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Nil(t, err)
}
