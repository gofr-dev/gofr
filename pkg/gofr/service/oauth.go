package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
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

	// last holds the most recently fetched token for a lock-free fast path. A still-valid token is
	// returned without entering singleflight or spawning a goroutine. The zero value is a nil
	// pointer; oauth2.Token.Valid is nil-safe, so it simply falls through to the fetch path.
	last atomic.Pointer[oauth2.Token]
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

	source := &clientCredentialsTokenSource{config: clientCredentials, ctx: tokenFetchContext()}

	return &oauthTokenProvider{tokenSource: oauth2.ReuseTokenSource(nil, source)}
}

// clientCredentialsTokenSource mints an access token with the client-credentials grant and lets the
// surrounding ReuseTokenSource cache it until it is close to expiring.
//
// expires_in is RECOMMENDED, not required (RFC 6749 §5.1). A grant that omits it leaves Token.Expiry
// zero, which oauth2 treats as never expiring, so the caching source would pin that one token for
// the life of the process; if the server later revoked it, every call would fail at the auth step
// until a restart. Marking a zero-expiry token already expired keeps it out of the cache, so a
// server that omits expires_in gets a fresh grant per call — the behavior before this caching was
// added. A server that sends expires_in is unaffected.
type clientCredentialsTokenSource struct {
	config clientcredentials.Config
	ctx    context.Context
}

func (s *clientCredentialsTokenSource) Token() (*oauth2.Token, error) {
	token, err := s.config.Token(s.ctx)
	if err != nil {
		return nil, err
	}

	if token.Expiry.IsZero() {
		token.Expiry = time.Now().Add(-time.Second)
	}

	return token, nil
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
	headers = cloneHeaders(headers)

	if headers[AuthHeader] != "" {
		return nil, AuthErr{Message: headerExistsMessage(AuthHeader)}
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
	// Fast path: a still-valid token is returned without singleflight or a goroutine. Token.Valid is
	// nil-safe, so the zero value (no token fetched yet) and an expired token both fall through.
	if last := p.last.Load(); last.Valid() {
		return last, nil
	}

	ch := p.group.DoChan(tokenSingleflightKey, p.fetch)

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

// fetch mints a token through the shared source and publishes it for the fast path. A panic in the
// source would otherwise be re-raised by singleflight in a fresh goroutine (it runs the work in a
// goroutine and re-panics an escaping panic), past any request's recover, taking the whole process
// down. Recovering here turns it into an error the caller handles instead.
func (p *oauthTokenProvider) fetch() (val any, err error) {
	defer func() {
		if r := recover(); r != nil {
			val, err = nil, AuthErr{Message: fmt.Sprintf("panic while fetching token: %v", r)}
		}
	}()

	token, err := p.tokenSource.Token()
	if err != nil {
		return nil, err
	}

	p.last.Store(token)

	return token, nil
}
