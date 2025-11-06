package middleware

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Predefined errors for consistent error handling.
var (
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
func (p *OIDCAuthProvider) GetAuthMethod() AuthMethod {
	return JWTClaim
}

// ExtractAuthHeader extracts and validates the Bearer token, returns userinfo.
func (p *OIDCAuthProvider) ExtractAuthHeader(r *http.Request) (any, error) {
	authHeader := r.Header.Get(http.CanonicalHeaderKey("Authorization"))

	token, ok := strings.CutPrefix(authHeader, "Bearer ")
	if !ok {
		return nil, NewMissingAuthHeaderError(http.CanonicalHeaderKey("Authorization"))
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return nil, NewMissingAuthHeaderError(http.CanonicalHeaderKey("Authorization"))
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, p.UserInfoEndpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCreateRequest, err)
	}

	req.Header.Set(http.CanonicalHeaderKey("Authorization"), "Bearer "+token)

	if p.Client == nil {
		return nil, errors.New("http client not initialized in OIDCAuthProvider")
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUserInfoFetch, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: received status %d", ErrUserInfoBadStatus, resp.StatusCode)
	}

	var userInfo map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUserInfoJSON, err)
	}

	return userInfo, nil
}
