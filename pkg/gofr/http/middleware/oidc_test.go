package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newMockUserInfoServer creates a test server with custom response.
func newMockUserInfoServer(responseCode int, responseBody string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"missing token"}`))

			return
		}

		w.WriteHeader(responseCode)
		_, _ = w.Write([]byte(responseBody))
	}))
}

func TestOIDCAuthProvider_ExtractAuthHeader_Success(t *testing.T) {
	userInfoJSON := `{"email":"foo@example.com","sub":"abc123"}`

	ts := newMockUserInfoServer(http.StatusOK, userInfoJSON)
	defer ts.Close()
	provider := &OIDCAuthProvider{UserInfoEndpoint: ts.URL}

	req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	req.Header.Set("Authorization", "Bearer validtoken")

	claims, err := provider.ExtractAuthHeader(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userInfo, ok := claims.(map[string]any)
	if !ok {
		t.Fatal("expected userInfo to be a map[string]interface{}")
	}

	if email, exists := userInfo["email"]; !exists || email != "foo@example.com" {
		t.Errorf("expected email foo@example.com, got %v", email)
	}
}

func TestOIDCAuthProvider_ExtractAuthHeader_MissingToken(t *testing.T) {
	provider := &OIDCAuthProvider{UserInfoEndpoint: "https://dummy"}
	req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody) // No Authorization header

	_, err := provider.ExtractAuthHeader(req)
	if errMissing, ok := err.(interface{ StatusCode() int }); !ok || errMissing.StatusCode() != http.StatusUnauthorized {
        t.Errorf("expected unauthorized error for missing token, got %v", err)
    }
}

func TestOIDCAuthProvider_ExtractAuthHeader_EmptyToken(t *testing.T) {
	provider := &OIDCAuthProvider{UserInfoEndpoint: "https://dummy"}
	req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	req.Header.Set("Authorization", "Bearer ")

	_, err := provider.ExtractAuthHeader(req)
	if errEmpty, ok := err.(interface{ StatusCode() int }); !ok || errEmpty.StatusCode() != http.StatusUnauthorized {
        t.Errorf("expected unauthorized error for empty token, got %v", err)
    }
}

func TestOIDCAuthProvider_ExtractAuthHeader_UserInfoEndpointError(t *testing.T) {
	ts := newMockUserInfoServer(http.StatusUnauthorized, `{"error":"unauthorized"}`)
	defer ts.Close()
	provider := &OIDCAuthProvider{UserInfoEndpoint: ts.URL}

	req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	req.Header.Set("Authorization", "Bearer validtoken")

	_, err := provider.ExtractAuthHeader(req)
	if !errors.Is(err, ErrUserInfoBadStatus) {
		t.Errorf("expected ErrUserInfoBadStatus, got %v", err)
	}
}

func TestOIDCAuthProvider_ExtractAuthHeader_BadUserInfoJSON(t *testing.T) {
	ts := newMockUserInfoServer(http.StatusOK, "{bad json")
	defer ts.Close()
	provider := &OIDCAuthProvider{UserInfoEndpoint: ts.URL}

	req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	req.Header.Set("Authorization", "Bearer validtoken")

	_, err := provider.ExtractAuthHeader(req)
	if !errors.Is(err, ErrUserInfoJSON) {
		t.Errorf("expected ErrUserInfoJSON, got %v", err)
	}
}
