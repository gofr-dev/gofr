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
	tokenSource oauth2.TokenSource
}

func (c *oAuthConfig) GetHeaderKey() string {
	return service.AuthHeader
}

func (c *oAuthConfig) GetHeaderValue(ctx context.Context) (string, error) {
	token, err := c.tokenSource.Token()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s %s", token.Type(), token.AccessToken), nil
}

// NewOAuthConfig validates the provided OAuth2 client credentials and returns a service.Options
// that injects Bearer tokens into outgoing HTTP requests.
func NewOAuthConfig(clientID, secret, tokenURL string, scopes []string,
	params url.Values, authStyle oauth2.AuthStyle) (service.Options, error) {
	if clientID == "" {
		return nil, AuthErr{nil, "client id is required"}
	}

	if secret == "" {
		return nil, AuthErr{nil, "client secret is required"}
	}

	if err := validateTokenURL(tokenURL); err != nil {
		return nil, err
	}

	cc := clientcredentials.Config{
		ClientID:       clientID,
		ClientSecret:   secret,
		TokenURL:       tokenURL,
		Scopes:         scopes,
		EndpointParams: params,
		AuthStyle:      authStyle,
	}

	config := &oAuthConfig{
		tokenSource: cc.TokenSource(context.Background()),
	}

	return NewAuthOption(config), nil
}

func validateTokenURL(tokenURL string) error {
	if tokenURL == "" {
		return AuthErr{nil, "token url is required"}
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
