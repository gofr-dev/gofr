package auth

import (
	"context"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"gofr.dev/pkg/gofr/service"
)

type oAuthTokenSource struct {
	source oauth2.TokenSource
}

func (o *oAuthTokenSource) Token(_ context.Context) (string, error) {
	token, err := o.source.Token()
	if err != nil {
		return "", err
	}

	return token.AccessToken, nil
}

// NewOAuthConfig validates the provided OAuth2 client credentials and returns a service.Options
// that injects Bearer tokens into outgoing HTTP requests.
func NewOAuthConfig(clientID, secret, tokenURL string, scopes []string,
	params url.Values, authStyle oauth2.AuthStyle) (service.Options, error) {
	if clientID == "" {
		return nil, Err{nil, "client id is required"}
	}

	if secret == "" {
		return nil, Err{nil, "client secret is required"}
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

	return NewBearerAuthOption(&oAuthTokenSource{
		source: cc.TokenSource(context.Background()),
	}), nil
}

func validateTokenURL(tokenURL string) error {
	if tokenURL == "" {
		return Err{nil, "token url is required"}
	}

	u, err := url.Parse(tokenURL)

	switch {
	case err != nil:
		return Err{err, "error in token URL"}
	case u.Host == "" || u.Scheme == "":
		return Err{err, "empty host"}
	case strings.Contains(u.Host, ".."):
		return Err{nil, "invalid host pattern, contains `..`"}
	case strings.HasSuffix(u.Host, "."):
		return Err{nil, "invalid host pattern, ends with `.`"}
	case u.Scheme != "http" && u.Scheme != "https":
		return Err{nil, "invalid scheme, allowed http and https only"}
	default:
		return nil
	}
}
