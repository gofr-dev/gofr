package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBodySizeLimit_WithinLimit(t *testing.T) {
	maxSize := int64(1024) // 1 KB
	body := bytes.NewReader(make([]byte, 512)) // 512 bytes

	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.ContentLength = 512
	rr := httptest.NewRecorder()

	handler := BodySizeLimit(maxSize)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "success", rr.Body.String())
}

func TestBodySizeLimit_ExceedsLimit_ContentLength(t *testing.T) {
	maxSize := int64(1024) // 1 KB
	body := bytes.NewReader(make([]byte, 2048)) // 2 KB

	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.ContentLength = 2048
	rr := httptest.NewRecorder()

	handler := BodySizeLimit(maxSize)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	assert.Contains(t, rr.Body.String(), "request body too large")
	assert.Contains(t, rr.Body.String(), "2048")
	assert.Contains(t, rr.Body.String(), "1024")
}

func TestBodySizeLimit_ExceedsLimit_ReadingBody(t *testing.T) {
	maxSize := int64(1024) // 1 KB
	body := bytes.NewReader(make([]byte, 2048)) // 2 KB

	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.ContentLength = -1 // Unknown content length
	rr := httptest.NewRecorder()

	handler := BodySizeLimit(maxSize)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
}

func TestBodySizeLimit_ZeroMaxSize_UsesDefault(t *testing.T) {
	body := bytes.NewReader(make([]byte, 100))

	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.ContentLength = 100
	rr := httptest.NewRecorder()

	handler := BodySizeLimit(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestBodySizeLimit_NegativeMaxSize_UsesDefault(t *testing.T) {
	body := bytes.NewReader(make([]byte, 100))

	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.ContentLength = 100
	rr := httptest.NewRecorder()

	handler := BodySizeLimit(-1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestBodySizeLimit_GET_NoCheck(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler := BodySizeLimit(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestBodySizeLimit_HEAD_NoCheck(t *testing.T) {
	req := httptest.NewRequest(http.MethodHead, "/test", nil)
	rr := httptest.NewRecorder()

	handler := BodySizeLimit(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestBodySizeLimit_DELETE_NoCheck(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/test", nil)
	rr := httptest.NewRecorder()

	handler := BodySizeLimit(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestErrorRequestBodyTooLarge(t *testing.T) {
	err := NewRequestBodyTooLargeError(1024, 2048)

	assert.Equal(t, "request body too large: 2048 bytes exceeds maximum allowed size of 1024 bytes", err.Error())
	assert.Equal(t, http.StatusRequestEntityTooLarge, err.StatusCode())
}

func TestBodySizeLimit_ExactLimit(t *testing.T) {
	maxSize := int64(1024) // 1 KB
	body := bytes.NewReader(make([]byte, 1024)) // Exactly 1 KB

	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.ContentLength = 1024
	rr := httptest.NewRecorder()

	handler := BodySizeLimit(maxSize)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestBodySizeLimit_PUT_Method(t *testing.T) {
	maxSize := int64(1024)
	body := bytes.NewReader(make([]byte, 2048))

	req := httptest.NewRequest(http.MethodPut, "/test", body)
	req.ContentLength = 2048
	rr := httptest.NewRecorder()

	handler := BodySizeLimit(maxSize)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
}

func TestBodySizeLimit_PATCH_Method(t *testing.T) {
	maxSize := int64(1024)
	body := bytes.NewReader(make([]byte, 512))

	req := httptest.NewRequest(http.MethodPatch, "/test", body)
	req.ContentLength = 512
	rr := httptest.NewRecorder()

	handler := BodySizeLimit(maxSize)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestBodySizeLimit_ResponseFormat(t *testing.T) {
	maxSize := int64(100)
	body := bytes.NewReader(make([]byte, 200))

	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.ContentLength = 200
	rr := httptest.NewRecorder()

	handler := BodySizeLimit(maxSize)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var response map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(http.StatusRequestEntityTooLarge), response["code"])
	assert.Equal(t, "ERROR", response["status"])
	assert.Contains(t, response["message"], "request body too large")
}

func TestBodySizeLimit_UnknownContentLength(t *testing.T) {
	maxSize := int64(100)
	body := bytes.NewReader(make([]byte, 50))

	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.ContentLength = -1 // Unknown
	rr := httptest.NewRecorder()

	handler := BodySizeLimit(maxSize)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the body - should succeed as it's within limit
		_, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

