package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/sync/singleflight"
)

const (
	msgClientIDMandatory     = "client id is mandatory"
	msgClientSecretMandatory = "client secret is mandatory" // #nosec G101
	msgTokenURLMandatory     = "token url is mandatory"     // #nosec G101
	msgEmptyHost             = "empty host"
	msgInvalidHostDotDot     = "invalid host pattern, contains `..`"
	msgInvalidHostEndsDot    = "invalid host pattern, ends with `.`"
	msgInvalidScheme         = "invalid scheme, allowed http and https only"

	// tokenRequestTimeout bounds a single call to the token endpoint. The token source is shared
	// across requests, so it cannot borrow a caller's deadline and needs a bound of its own.
	tokenRequestTimeout = 10 * time.Second
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
	return &authProvider{auth: c.newTokenProvider().addAuthorizationHeader, HTTP: svc}
}

func NewOAuthConfig(clientID, secret, tokenURL string, scopes []string, params url.Values, authStyle oauth2.AuthStyle) (Options, error) {
	if clientID == "" {
		return nil, AuthErr{nil, msgClientIDMandatory}
	}

	if secret == "" {
		return nil, AuthErr{nil, msgClientSecretMandatory}
	}

	if err := validateTokenURL(tokenURL); err != nil {
		return nil, err
	}

	config := OAuthConfig{
		ClientID:       clientID,
		ClientSecret:   secret,
		TokenURL:       tokenURL,
		Scopes:         scopes,
		EndpointParams: params,
		AuthStyle:      authStyle,
	}

	return &config, nil
}

func validateTokenURL(tokenURL string) error {
	if tokenURL == "" {
		return AuthErr{nil, msgTokenURLMandatory}
	}

	u, err := url.Parse(tokenURL)

	switch {
	case err != nil:
		return AuthErr{err, "error in token URL"}
	case u.Host == "" || u.Scheme == "":
		return AuthErr{err, msgEmptyHost}
	case strings.Contains(u.Host, ".."):
		return AuthErr{nil, msgInvalidHostDotDot}
	case strings.HasSuffix(u.Host, "."):
		return AuthErr{nil, msgInvalidHostEndsDot}
	case u.Scheme != methodHTTP && u.Scheme != methodHTTPS:
		return AuthErr{nil, msgInvalidScheme}
	default:
		return nil
	}
}

// oauthTokenProvider holds the token source for one OAuthConfig registration.
//
// clientcredentials.Config.TokenSource returns a source that caches the token until it is close to
// expiring, but the cache lives in the source it returns and not in the Config. Building a source
// per request discarded that cache, so every outgoing call minted a new access token.
//
// The source's own lock serializes callers while a fetch is in flight, so a hung token endpoint
// would otherwise queue every caller behind it, each for a full tokenRequestTimeout. group
// collapses concurrent callers onto the one fetch already running, and token still lets each
// caller leave on its own context instead of waiting for that fetch to finish.
type oauthTokenProvider struct {
	tokenSource oauth2.TokenSource
	group       singleflight.Group
}

// newTokenProvider builds the caching token source for a config. Like the other options in this
// package, the config is read once when the option is applied and not on every request.
func (c *OAuthConfig) newTokenProvider() *oauthTokenProvider {
	clientCredentials := clientcredentials.Config{
		ClientID:       c.ClientID,
		ClientSecret:   c.ClientSecret,
		TokenURL:       c.TokenURL,
		Scopes:         c.Scopes,
		EndpointParams: c.EndpointParams,
		AuthStyle:      c.AuthStyle,
	}

	return &oauthTokenProvider{tokenSource: clientCredentials.TokenSource(tokenFetchContext())}
}

// tokenFetchContext returns the context the token source uses to reach the authorization server.
// The source is shared by every request, so binding it to one caller would let that caller's
// cancellation decide whether later requests can get a token. The oauth2 client is given an
// explicit timeout instead, to keep the fetch bounded.
func tokenFetchContext() context.Context {
	return context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Timeout: tokenRequestTimeout})
}

// addAuthorizationHeader sets the bearer token on headers, reusing the cached token while it is
// still valid. If a fetch is needed and ctx is done first, the caller leaves with ctx.Err() rather
// than waiting for the fetch to finish; see token.
func (p *oauthTokenProvider) addAuthorizationHeader(ctx context.Context, headers map[string]string) (map[string]string, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	if authHeader, ok := headers[AuthHeader]; ok && authHeader != "" {
		return nil, AuthErr{Message: fmt.Sprintf("value %v already exists for header %v", authHeader, AuthHeader)}
	}

	token, err := p.token(ctx)
	if err != nil {
		return nil, err
	}

	headers[AuthHeader] = fmt.Sprintf("%v %v", token.Type(), token.AccessToken)

	return headers, nil
}

// tokenSingleflightKey names the single in-flight token fetch a provider ever has. There is only
// one, so the key is a constant; it exists only because singleflight is keyed by design.
const tokenSingleflightKey = "token"

// token returns the cached token, or fetches one when it has expired. Concurrent callers arriving
// while a fetch is already running join that same fetch through group rather than each taking a
// turn at the token source's lock, so a hung token endpoint costs one tokenRequestTimeout for the
// whole burst rather than one per caller.
//
// A caller whose ctx is done before the fetch returns leaves with ctx.Err() and does not wait for
// it. The fetch itself keeps running on tokenFetchContext, unaffected by any caller leaving
// early, and still populates the cache for whoever calls next. A failed fetch is not cached, so a
// transient authorization-server failure is retried on the next call rather than remembered.
func (p *oauthTokenProvider) token(ctx context.Context) (*oauth2.Token, error) {
	ch := p.group.DoChan(tokenSingleflightKey, func() (any, error) {
		return p.tokenSource.Token()
	})

	select {
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}

		token, _ := res.Val.(*oauth2.Token)

		return token, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
