// File: pkg/gofr/http/middleware/discovery.go

package middleware

import (
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "time"
)

// OIDCMetadata represents the parts of the OIDC discovery document you need.
type OIDCMetadata struct {
    Issuer           string `json:"issuer"`
    JWKSURI          string `json:"jwks_uri"`
    UserInfoEndpoint string `json:"userinfo_endpoint"`
}

var (
    cachedMeta    *OIDCMetadata
    cacheExpiry   time.Time
    cacheDuration = 10 * time.Minute // Adjust cache TTL as needed
    mu            sync.Mutex
)

// FetchOIDCMetadata fetches and caches OIDC discovery metadata from the given URL.
// It returns cached data if within cache duration.
func FetchOIDCMetadata(discoveryURL string) (*OIDCMetadata, error) {
    mu.Lock()
    defer mu.Unlock()

    // Return cached metadata if still valid
    if cachedMeta != nil && time.Now().Before(cacheExpiry) {
        return cachedMeta, nil
    }

    resp, err := http.Get(discoveryURL)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch OIDC discovery metadata: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("OIDC discovery: unexpected HTTP status %d", resp.StatusCode)
    }

    var meta OIDCMetadata
    if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
        return nil, fmt.Errorf("failed to decode OIDC discovery JSON: %w", err)
    }

    // Cache the fetched metadata
    cachedMeta = &meta
    cacheExpiry = time.Now().Add(cacheDuration)

    return &meta, nil
}

