package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// Predefined errors for consistent error handling.
var (
	ErrMissingToken      = errors.New("missing bearer token")
	ErrEmptyToken        = errors.New("empty bearer token")
	ErrCreateRequest     = errors.New("failed to create userinfo request")
	ErrUserInfoFetch     = errors.New("failed to fetch userinfo")
	ErrUserInfoBadStatus = errors.New("userinfo endpoint returned error status")
	ErrUserInfoJSON      = errors.New("failed to parse userinfo response")
)

// OIDCAuthProvider implements the AuthProvider interface for OIDC.
type OIDCAuthProvider struct {
	UserInfoEndpoint string
	Client           *http.Client
}

// GetAuthMethod returns the authentication method for OIDCAuthProvider.
func (p *OIDCAuthProvider) GetAuthMethod() authMethod {
	return JWTClaim
}

// ExtractAuthHeader extracts and validates the Bearer token, returns userinfo.
func (p *OIDCAuthProvider) ExtractAuthHeader(r *http.Request) (any, error) {
	authHeader := r.Header.Get("Authorization")

	token, ok := strings.CutPrefix(authHeader, "Bearer ")
	if !ok {
		return nil, ErrMissingToken
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrEmptyToken
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, p.UserInfoEndpoint, http.NoBody)
	if err != nil {
		return nil, ErrCreateRequest
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, ErrUserInfoFetch
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrUserInfoBadStatus
	}

	var userInfo map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, ErrUserInfoJSON
	}

	return userInfo, nil
}
