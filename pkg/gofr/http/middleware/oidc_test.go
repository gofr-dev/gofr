package middleware

import (
    "context"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

// Helper to create a mock userinfo endpoint server
func newMockUserInfoServer(responseCode int, responseBody string) *httptest.Server {
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        auth := r.Header.Get("Authorization")
        if !strings.HasPrefix(auth, "Bearer ") {
            w.WriteHeader(http.StatusUnauthorized)
            w.Write([]byte(`{"error":"missing token"}`))
            return
        }
        w.WriteHeader(responseCode)
        w.Write([]byte(responseBody))
    }))
}

func TestOIDCUserInfoMiddleware_Success(t *testing.T) {
    userInfoJSON := `{"email":"foo@example.com","sub":"abc123"}`
    ts := newMockUserInfoServer(http.StatusOK, userInfoJSON)
    defer ts.Close()

    mw := OIDCUserInfoMiddleware(ts.URL)

    handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        userInfo, ok := GetOIDCUserInfo(r.Context())
        if !ok {
            t.Fatal("userInfo not found in context")
        }
        if email, exists := userInfo["email"]; !exists || email != "foo@example.com" {
            t.Errorf("expected email foo@example.com, got %v", email)
        }
        w.WriteHeader(http.StatusOK)
    }))

    req := httptest.NewRequest("GET", "/", nil)
    req.Header.Set("Authorization", "Bearer validtoken")
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)

    if rr.Code != http.StatusOK {
        t.Fatalf("expected status OK, got %v", rr.Code)
    }
}

func TestOIDCUserInfoMiddleware_MissingBearerToken(t *testing.T) {
    mw := OIDCUserInfoMiddleware("https://dummy")

    handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        t.Fatal("handler should not be called if token missing")
    }))

    req := httptest.NewRequest("GET", "/", nil)
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)

    if rr.Code != http.StatusUnauthorized {
        t.Errorf("expected unauthorized status code, got %d", rr.Code)
    }
}

func TestOIDCUserInfoMiddleware_BadUserInfoResponse(t *testing.T) {
    // Server returns invalid JSON
    ts := newMockUserInfoServer(http.StatusOK, "{bad json")
    defer ts.Close()

    mw := OIDCUserInfoMiddleware(ts.URL)
    handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        t.Fatal("handler should not be called if userinfo JSON invalid")
    }))

    req := httptest.NewRequest("GET", "/", nil)
    req.Header.Set("Authorization", "Bearer validtoken")
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)

    if rr.Code != http.StatusInternalServerError {
        t.Errorf("expected internal server error status code for bad JSON, got %d", rr.Code)
    }
}

func TestOIDCUserInfoMiddleware_UserInfoEndpointReturnsError(t *testing.T) {
    // Server returns 401 unauthorized
    ts := newMockUserInfoServer(http.StatusUnauthorized, `{"error":"unauthorized"}`)
    defer ts.Close()

    mw := OIDCUserInfoMiddleware(ts.URL)
    handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        t.Fatal("handler should not be called if userinfo endpoint errors")
    }))

    req := httptest.NewRequest("GET", "/", nil)
    req.Header.Set("Authorization", "Bearer validtoken")
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)

    if rr.Code != http.StatusUnauthorized {
        t.Errorf("expected unauthorized status code for userinfo error, got %d", rr.Code)
    }
}

func TestGetOIDCUserInfo_Helper(t *testing.T) {
    ctx := context.WithValue(context.Background(), userInfoKey, map[string]interface{}{"foo": "bar"})

    userInfo, ok := GetOIDCUserInfo(ctx)
    if !ok {
        t.Error("expected true, got false")
    }
    if val, exists := userInfo["foo"]; !exists || val != "bar" {
        t.Errorf("got %v, want bar", val)
    }

    // Test with missing key
    emptyCtx := context.Background()
    _, ok = GetOIDCUserInfo(emptyCtx)
    if ok {
        t.Error("expected false, got true")
    }
}

