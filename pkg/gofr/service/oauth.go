package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// OAuthConfig describes a 2-legged OAuth2 flow, with both the
// client application information and the server's endpoint URLs.
type OAuthConfig struct {
	// ClientID is the application's ID.
	ClientID string

	// ClientSecret is the application's secret.
	ClientSecret string

	// TokenURL is the resource server's token endpoint
	// URL. This is a constant specific to each server.
	TokenURL string

	// Scope specifies optional requested permissions.
	Scopes []string

	// EndpointParams specifies additional parameters for requests to the token endpoint.
	EndpointParams url.Values

	// AuthStyle represents how requests for tokens are authenticated to the server
	// Defaults to [oauth2.AuthStyleAutoDetect]
	AuthStyle oauth2.AuthStyle
}

func (c *OAuthConfig) AddOption(svc HTTP) HTTP {
	return &authProvider{auth: c.addAuthorizationHeader, HTTP: svc}
}

// Validate implements the Validator interface for OAuthConfig.
// Returns an error if clientID, secret, or tokenURL is invalid.
func (c *OAuthConfig) Validate() error {
	if c.ClientID == "" {
		return AuthErr{nil, "client id is mandatory"}
	}

	if c.ClientSecret == "" {
		return AuthErr{nil, "client secret is mandatory"}
	}

	if err := validateTokenURL(c.TokenURL); err != nil {
		return err
	}

	return nil
}

// FeatureName implements the Validator interface.
func (*OAuthConfig) FeatureName() string {
	return "OAuth2 Authentication"
}

func NewOAuthConfig(clientID, secret, tokenURL string, scopes []string, params url.Values, authStyle oauth2.AuthStyle) (Options, error) {
	if clientID == "" {
		return nil, AuthErr{nil, "client id is mandatory"}
	}

	if secret == "" {
		return nil, AuthErr{nil, "client secret is mandatory"}
	}

	if err := validateTokenURL(tokenURL); err != nil {
		return nil, err
	}

	config := &OAuthConfig{
		ClientID:       clientID,
		ClientSecret:   secret,
		TokenURL:       tokenURL,
		Scopes:         scopes,
		EndpointParams: params,
		AuthStyle:      authStyle,
	}

	// Validate during creation as well for immediate feedback
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

func validateTokenURL(tokenURL string) error {
	if tokenURL == "" {
		return AuthErr{nil, "token url is mandatory"}
	}

	u, err := url.Parse(tokenURL)

	switch {
	case err != nil:
		return AuthErr{err, "error in token URL"}
	case u.Host == "" || u.Scheme == "":
		return AuthErr{err, "empty host"}
	case strings.Contains(u.Host, ".."):
		return AuthErr{nil, "invalid host pattern, contains `..`"}
	case strings.HasSuffix(u.Host, "."):
		return AuthErr{nil, "invalid host pattern, ends with `.`"}
	case u.Scheme != "http" && u.Scheme != "https":
		return AuthErr{nil, "invalid scheme, allowed http and https only"}
	default:
		return nil
	}
}

func (c *OAuthConfig) addAuthorizationHeader(ctx context.Context, headers map[string]string) (map[string]string, error) {
	var err error

	if headers == nil {
		headers = make(map[string]string)
	}

	if authHeader, ok := headers[AuthHeader]; ok && authHeader != "" {
		return nil, AuthErr{Message: fmt.Sprintf("value %v already exists for header %v", authHeader, AuthHeader)}
	}

	clientCredentials := clientcredentials.Config{
		ClientID:       c.ClientID,
		ClientSecret:   c.ClientSecret,
		TokenURL:       c.TokenURL,
		Scopes:         c.Scopes,
		EndpointParams: c.EndpointParams,
		AuthStyle:      c.AuthStyle,
	}

	token, err := clientCredentials.TokenSource(ctx).Token()
	if err != nil {
		return nil, err
	}

	headers[AuthHeader] = fmt.Sprintf("%v %v", token.Type(), token.AccessToken)

	return headers, nil
}
