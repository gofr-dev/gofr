package middleware

import (
    "net/http"
    "net/http/httptest"
    "sync"
    "testing"
    "time"
)

func resetCache() {
    // Helper to reset package-level cache for isolated tests
    mu.Lock()
    defer mu.Unlock()
    cachedMeta = nil
    cacheExpiry = time.Time{}
}

func TestFetchOIDCMetadata_Success(t *testing.T) {
    sampleJSON := `{
        "issuer": "https://example.com",
        "jwks_uri": "https://example.com/jwks.json",
        "userinfo_endpoint": "https://example.com/userinfo"
    }`

    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(sampleJSON))
    }))
    defer server.Close()

    resetCache()

    meta, err := FetchOIDCMetadata(server.URL)
    if err != nil {
        t.Fatalf("expected no error, got %v", err)
    }
    if meta.Issuer != "https://example.com" {
        t.Errorf("expected issuer 'https://example.com', got %q", meta.Issuer)
    }
    if meta.JWKSURI != "https://example.com/jwks.json" {
        t.Errorf("expected jwks_uri 'https://example.com/jwks.json', got %q", meta.JWKSURI)
    }
    if meta.UserInfoEndpoint != "https://example.com/userinfo" {
        t.Errorf("expected userinfo_endpoint 'https://example.com/userinfo', got %q", meta.UserInfoEndpoint)
    }
}

func TestFetchOIDCMetadata_Caching(t *testing.T) {
    sampleJSON := `{
        "issuer": "https://example.org",
        "jwks_uri": "https://example.org/jwks.json",
        "userinfo_endpoint": "https://example.org/userinfo"
    }`

    var requestCount int
    var muRequest sync.Mutex

    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        muRequest.Lock()
        requestCount++
        muRequest.Unlock()
        w.Write([]byte(sampleJSON))
    }))
    defer server.Close()

    resetCache()

    // First call: triggers HTTP request
    meta1, err1 := FetchOIDCMetadata(server.URL)
    if err1 != nil {
        t.Fatalf("unexpected error on first fetch: %v", err1)
    }

    // Second call immediately: should return cached metadata without HTTP request
    meta2, err2 := FetchOIDCMetadata(server.URL)
    if err2 != nil {
        t.Fatalf("unexpected error on second fetch: %v", err2)
    }

    muRequest.Lock()
    count := requestCount
    muRequest.Unlock()

    if count != 1 {
        t.Errorf("expected 1 HTTP request due to caching, got %d", count)
    }

    if meta1 != meta2 {
        t.Error("expected cached metadata to be returned on second call")
    }

    // Simulate cache expiry by adjusting cacheExpiry directly
    mu.Lock()
    cacheExpiry = time.Now().Add(-time.Minute) // expired
    mu.Unlock()

    // Third call after expiry: triggers http request again
    _, err3 := FetchOIDCMetadata(server.URL)
    if err3 != nil {
        t.Fatalf("unexpected error on third fetch after expiry: %v", err3)
    }

    muRequest.Lock()
    updatedCount := requestCount
    muRequest.Unlock()

    if updatedCount != 2 {
        t.Errorf("expected 2 HTTP requests after expiry, got %d", updatedCount)
    }
}

func TestFetchOIDCMetadata_Non200Status(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(`Bad request`))
    }))
    defer server.Close()

    resetCache()

    _, err := FetchOIDCMetadata(server.URL)
    if err == nil {
        t.Fatal("expected error on non-200 HTTP status, got nil")
    }
}

func TestFetchOIDCMetadata_BadJSON(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("{invalid json}"))
    }))
    defer server.Close()

    resetCache()

    _, err := FetchOIDCMetadata(server.URL)
    if err == nil {
        t.Fatal("expected error due to bad JSON, got nil")
    }
}

func TestFetchOIDCMetadata_HTTPError(t *testing.T) {
    resetCache()

    // Using an invalid URL to force HTTP client error
    _, err := FetchOIDCMetadata("http://invalid.invalid")
    if err == nil {
        t.Fatal("expected error due to HTTP GET failure, got nil")
    }
}

