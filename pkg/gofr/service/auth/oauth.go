package auth

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"gofr.dev/pkg/gofr/service"
)

type oAuthConfig struct {
	clientID       string
	clientSecret   string
	tokenURL       string
	scopes         []string
	endpointParams url.Values
	authStyle      oauth2.AuthStyle
}

// GetHeaderKey returns the Authorization header key.
func (c *oAuthConfig) GetHeaderKey() string {
	return AuthHeader
}

// GetHeaderValue performs the OAuth2 client credentials exchange and returns the bearer token.
func (c *oAuthConfig) GetHeaderValue(ctx context.Context) (string, error) {
	cc := clientcredentials.Config{
		ClientID:       c.clientID,
		ClientSecret:   c.clientSecret,
		TokenURL:       c.tokenURL,
		Scopes:         c.scopes,
		EndpointParams: c.endpointParams,
		AuthStyle:      c.authStyle,
	}

	token, err := cc.TokenSource(ctx).Token()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%v %v", token.Type(), token.AccessToken), nil
}

// NewOAuthConfig validates the provided OAuth2 client credentials and returns a service.Options
// that injects Bearer tokens into outgoing HTTP requests.
func NewOAuthConfig(clientID, secret, tokenURL string, scopes []string,
	params url.Values, authStyle oauth2.AuthStyle) (service.Options, error) {
	if clientID == "" {
		return nil, AuthErr{nil, "client id is mandatory"}
	}

	if secret == "" {
		return nil, AuthErr{nil, "client secret is mandatory"}
	}

	if err := validateTokenURL(tokenURL); err != nil {
		return nil, err
	}

	config := &oAuthConfig{
		clientID:       clientID,
		clientSecret:   secret,
		tokenURL:       tokenURL,
		scopes:         scopes,
		endpointParams: params,
		authStyle:      authStyle,
	}

	return NewAuthOption(config), nil
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
