package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"
)

// Predefined errors for discovery metadata fetching.
var (
	ErrFailedCreateDiscoveryRequest = errors.New("failed to create OIDC discovery request")
	ErrFailedFetchDiscoveryMetadata = errors.New("failed to fetch OIDC discovery metadata")
	ErrBadDiscoveryStatus           = errors.New("OIDC discovery: unexpected HTTP status")
	ErrFailedDecodeDiscoveryJSON    = errors.New("failed to decode OIDC discovery JSON")
)

// OIDCMetadata represents the parts of the OIDC discovery document you need.
type OIDCMetadata struct {
	Issuer           string `json:"issuer"`
	JWKSURI          string `json:"jwks_uri"`
	UserInfoEndpoint string `json:"userinfo_endpoint"`
}

// DiscoveryCache caches OIDC metadata per discovery URL.
type DiscoveryCache struct {
	url           string
	cacheDuration time.Duration
	mu            sync.Mutex
	cachedMeta    *OIDCMetadata
	cacheExpiry   time.Time
}

// NewDiscoveryCache creates a new per-URL OIDC discovery cache.
func NewDiscoveryCache(url string, cacheDuration time.Duration) *DiscoveryCache {
	return &DiscoveryCache{
		url:           url,
		cacheDuration: cacheDuration,
	}
}

// GetMetadata fetches and caches OIDC discovery metadata from this cache's URL.
func (dc *DiscoveryCache) GetMetadata(ctx context.Context) (*OIDCMetadata, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	// Return cached metadata if still valid
	if dc.cachedMeta != nil && time.Now().Before(dc.cacheExpiry) {
		return dc.cachedMeta, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dc.url, http.NoBody)
	if err != nil {
		return nil, ErrFailedCreateDiscoveryRequest
	}

	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return nil, ErrFailedFetchDiscoveryMetadata
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrBadDiscoveryStatus
	}

	var meta OIDCMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, ErrFailedDecodeDiscoveryJSON
	}

	dc.cachedMeta = &meta
	dc.cacheExpiry = time.Now().Add(dc.cacheDuration)

	return &meta, nil
}
