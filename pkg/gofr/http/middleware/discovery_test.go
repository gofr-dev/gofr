package middleware

import (
    "context"
    "net/http"
    "net/http/httptest"
    "sync"
    "testing"
    "time"
)

// TestFetchOIDCMetadata_Success verifies fetching and parsing of metadata.
func TestDiscoveryCache_GetMetadata_Success(t *testing.T) {
    sampleJSON := `{
        "issuer": "https://example.com",
        "jwks_uri": "https://example.com/jwks.json",
        "userinfo_endpoint": "https://example.com/userinfo"
    }`

    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte(sampleJSON))
    }))
    defer server.Close()

    cache := NewDiscoveryCache(server.URL, 10*time.Minute)

    meta, err := cache.GetMetadata(context.Background())
    if err != nil {
        t.Fatalf("expected no error, got %v", err)
    }

    if meta.Issuer != "https://example.com" {
        t.Errorf("expected issuer https://example.com, got %q", meta.Issuer)
    }
    if meta.JWKSURI != "https://example.com/jwks.json" {
        t.Errorf("expected jwks_uri https://example.com/jwks.json, got %q", meta.JWKSURI)
    }
    if meta.UserInfoEndpoint != "https://example.com/userinfo" {
        t.Errorf("expected userinfo_endpoint https://example.com/userinfo, got %q", meta.UserInfoEndpoint)
    }
}

// TestDiscoveryCache_Caching verifies subsequent calls within TTL do not re-fetch.
func TestDiscoveryCache_Caching(t *testing.T) {
    sampleJSON := `{
        "issuer": "https://example.org",
        "jwks_uri": "https://example.org/jwks.json",
        "userinfo_endpoint": "https://example.org/userinfo"
    }`

    var requestCount int
    var muReq sync.Mutex

    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        muReq.Lock()
        requestCount++
        muReq.Unlock()
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte(sampleJSON))
    }))
    defer server.Close()

    cache := NewDiscoveryCache(server.URL, 10*time.Minute)

    // First call — should trigger HTTP fetch
    if _, err := cache.GetMetadata(context.Background()); err != nil {
        t.Fatalf("unexpected error on first fetch: %v", err)
    }

    // Second call within TTL — should hit cache
    if _, err := cache.GetMetadata(context.Background()); err != nil {
        t.Fatalf("unexpected error on cached fetch: %v", err)
    }

    muReq.Lock()
    if requestCount != 1 {
        t.Errorf("expected 1 HTTP request due to caching, got %d", requestCount)
    }
    muReq.Unlock()

    // Force expiry
    cache.mu.Lock()
    cache.cacheExpiry = time.Now().Add(-time.Minute)
    cache.mu.Unlock()

    // Third call — should trigger another HTTP fetch
    if _, err := cache.GetMetadata(context.Background()); err != nil {
        t.Fatalf("unexpected error on post-expiry fetch: %v", err)
    }

    muReq.Lock()
    if requestCount != 2 {
        t.Errorf("expected 2 HTTP requests after expiry, got %d", requestCount)
    }
    muReq.Unlock()
}

// TestDiscoveryCache_Non200Status ensures error for non-200 HTTP status.
func TestDiscoveryCache_Non200Status(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        http.Error(w, "bad request", http.StatusBadRequest)
    }))
    defer server.Close()

    cache := NewDiscoveryCache(server.URL, 10*time.Minute)

    _, err := cache.GetMetadata(context.Background())
    if err == nil {
        t.Fatal("expected error for non-200 status, got nil")
    }
}

// TestDiscoveryCache_BadJSON ensures error if JSON is malformed.
func TestDiscoveryCache_BadJSON(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("{bad json"))
    }))
    defer server.Close()

    cache := NewDiscoveryCache(server.URL, 10*time.Minute)

    _, err := cache.GetMetadata(context.Background())
    if err == nil {
        t.Fatal("expected error for bad JSON, got nil")
    }
}

// TestDiscoveryCache_HTTPError ensures network errors are returned.
func TestDiscoveryCache_HTTPError(t *testing.T) {
    // Use an invalid port to cause a connection error
    cache := NewDiscoveryCache("http://127.0.0.1:0", 10*time.Minute)

    _, err := cache.GetMetadata(context.Background())
    if err == nil {
        t.Fatal("expected error due to HTTP failure, got nil")
    }
}

