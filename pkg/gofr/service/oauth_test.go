package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"gofr.dev/pkg/gofr/logging"
)

const invalidURL = "abc://invalid-url"

var (
	errMissingTokenURL    = errors.New(`unsupported protocol scheme ""`)
	errIncorrectProtocol  = errors.New(`unsupported protocol scheme "abc"`)
	errInvalidCredentials = &oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusUnauthorized}}
)

func TestNewOAuthConfig(t *testing.T) {
	config := oAuthConfigForTests(t, "/token")

	server := setupOAuthHTTPServer(t, config)

	tokenURL := server.URL + config.TokenURL
	clientID := config.ClientID
	clientSecret := config.ClientSecret

	testCases := []struct {
		clientID     string
		clientSecret string
		tokenURL     string
		scopes       []string
		params       url.Values
		authStyle    oauth2.AuthStyle
		err          error
	}{
		{err: AuthErr{nil, "client id is mandatory"}},
		{clientID: clientID, err: AuthErr{nil, "client secret is mandatory"}},
		{clientID: clientID, tokenURL: tokenURL, err: AuthErr{nil, "client secret is mandatory"}},
		{clientID: clientID, clientSecret: clientSecret, err: AuthErr{nil, "token url is mandatory"}},
		{clientID: clientID, clientSecret: clientSecret, tokenURL: "invalid_url_format", err: AuthErr{nil, "empty host"}},
		{clientID: clientID, clientSecret: clientSecret, tokenURL: tokenURL},
		{clientID: clientID, clientSecret: "some_random_client_secret", tokenURL: tokenURL},
		{clientID: "some_random_client_id", clientSecret: clientSecret, tokenURL: tokenURL},
		{clientID: clientID, clientSecret: clientSecret, tokenURL: tokenURL, authStyle: 1},
		{clientID: clientID, clientSecret: "some_random_client_secret", tokenURL: tokenURL, authStyle: 1},
		{clientID: "some_random_client_id", clientSecret: clientSecret, tokenURL: tokenURL, authStyle: 2},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test case #%d", i), func(t *testing.T) {
			config, err := NewOAuthConfig(tc.clientID, tc.clientSecret, tc.tokenURL, tc.scopes, tc.params, tc.authStyle)
			assert.Equal(t, tc.err, err)

			if tc.err != nil {
				assert.Empty(t, config)
				return
			}

			oAuthConfig, ok := config.(*OAuthConfig)
			assert.True(t, ok, "failed to get OAuthConfig")

			if oAuthConfig == nil {
				t.Errorf("failed to get OAuthConfig")
				return
			}

			assert.Equal(t, tc.clientID, oAuthConfig.ClientID)
			assert.Equal(t, tc.clientSecret, oAuthConfig.ClientSecret)
			assert.Equal(t, tc.tokenURL, oAuthConfig.TokenURL)
			assert.Equal(t, tc.params, oAuthConfig.EndpointParams)
			assert.Equal(t, tc.scopes, oAuthConfig.Scopes)
			assert.Equal(t, tc.authStyle, oAuthConfig.AuthStyle)
		})
	}
}

func TestHttpService_validateTokenURL(t *testing.T) {
	testCases := []struct {
		tokenURL string
		errMsg   string
	}{
		{tokenURL: "https://www.example.com"},
		{tokenURL: "https://www.example.com.", errMsg: "invalid host pattern, ends with `.`"},
		{tokenURL: "https://www.192.168.1.1.com"},
		{tokenURL: "https://www.192.168.1.1..com", errMsg: "invalid host pattern, contains `..`"},
		{tokenURL: "ftp://www.192.168.1.1..com", errMsg: "invalid host pattern, contains `..`"},
		{tokenURL: "ftp://www.192.168.1.1.com", errMsg: "invalid scheme, allowed http and https only"},
		{tokenURL: "www.192.168.1.1.com", errMsg: "empty host"},
		{tokenURL: "https://www.example.", errMsg: "invalid host pattern, ends with `.`"},
		{errMsg: "token url is mandatory"},
		{tokenURL: "invalid_url_format", errMsg: "empty host"},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test Case #%d", i), func(t *testing.T) {
			err := validateTokenURL(tc.tokenURL)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			}
		})
	}
}

func TestAddAuthorizationHeader_OAuth(t *testing.T) {
	config := oAuthConfigForTests(t, "/token")

	server := setupOAuthHTTPServer(t, config)

	tokenURL := server.URL + config.TokenURL

	emptyHeaders := map[string]string{}
	headerWithAuth := map[string]string{AuthHeader: "Value"}
	headerWithEmptyAuth := map[string]string{AuthHeader: ""}
	headerWithoutAuth := map[string]string{"Content Type": "Value"}
	headerWithEmptyAuthAndOtherValues := map[string]string{"Content Type": "Value", AuthHeader: ""}
	authHeaderExistsError := AuthErr{Message: "value Value already exists for header Authorization"}

	testCases := []struct {
		tokenURL string
		headers  map[string]string
		response map[string]string
		err      error
	}{
		{headers: headerWithAuth, err: authHeaderExistsError},
		{err: &url.Error{Op: "Post", URL: "", Err: errMissingTokenURL}},
		{tokenURL: tokenURL, headers: headerWithAuth, err: authHeaderExistsError},
		{tokenURL: tokenURL, headers: headerWithEmptyAuth, response: emptyHeaders},
		{tokenURL: tokenURL, headers: headerWithoutAuth, response: headerWithoutAuth},
		{tokenURL: tokenURL, headers: headerWithEmptyAuthAndOtherValues, response: headerWithoutAuth},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test Case #%d", i), func(t *testing.T) {
			config.TokenURL = tc.tokenURL

			// A provider per case mirrors AddOption, which snapshots the config into a token
			// source once per registration rather than reading it on every request.
			headers, err := config.newTokenProvider().addAuthorizationHeader(t.Context(), tc.headers)
			assert.Equal(t, tc.err, err)

			if err != nil {
				return
			}

			authHeader, ok := headers[AuthHeader]
			assert.True(t, ok)
			assert.NotEmpty(t, authHeader)
			assert.True(t, strings.HasPrefix(authHeader, "Bearer"))
			delete(headers, AuthHeader)
			assert.Equal(t, tc.response, headers)
		})
	}
}

// Helper method for getting OAuthConfig.
func oAuthConfigForTests(t *testing.T, tokenURL string) *OAuthConfig {
	t.Helper()

	config := &OAuthConfig{
		TokenURL: tokenURL,
		EndpointParams: map[string][]string{
			"aud": {"some-random-value"},
		},
		AuthStyle: oauth2.AuthStyleInParams,
	}

	clientID, err := generateRandomString(clientIDLength)
	if err != nil {
		t.Fatalf("unable to generate random string for oAuthConfig")
	}

	config.ClientID = clientID

	clientSecret, err := generateRandomString(clientSecretLength)
	if err != nil {
		t.Fatalf("unable to generate random string for oAuthConfig")
	}

	config.ClientSecret = clientSecret

	return config
}

// tokenLifetime is long enough for oauth2's caching source to treat a new token as usable.
// Anything at or below its 10s early-refresh window counts as already expired.
const tokenLifetime = 3600

// shortTokenLifetime sits inside that window, so such a token is never served from cache.
const shortTokenLifetime = 1

// countingTokenServer is a client_credentials token endpoint that counts the grants it issues and
// returns a distinct access token for each, so a test can tell a cached token from a new one.
type countingTokenServer struct {
	*httptest.Server

	grants    atomic.Int64
	rejects   atomic.Int64 // leading grant requests to answer with 503 instead of a token
	expiresIn int
}

func newCountingTokenServer(t *testing.T, expiresIn int) *countingTokenServer {
	t.Helper()

	ts := &countingTokenServer{expiresIn: expiresIn}

	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if ts.rejects.Load() > 0 {
			ts.rejects.Add(-1)
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		grant := ts.grants.Add(1)

		w.Header().Set("Content-Type", "application/json")

		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": fmt.Sprintf("token-%d", grant),
			"token_type":   "Bearer",
			"expires_in":   ts.expiresIn,
		})
	}))

	t.Cleanup(ts.Close)

	return ts
}

func (ts *countingTokenServer) oAuthConfig() *OAuthConfig {
	return &OAuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		TokenURL:     ts.URL,
		AuthStyle:    oauth2.AuthStyleInParams,
	}
}

// recordingUpstream answers every request with 200 and reports the Authorization header it saw on
// a buffered channel, so the test can read the headers safely under -race.
func recordingUpstream(t *testing.T, capacity int) (srv *httptest.Server, seen <-chan string) {
	t.Helper()

	headers := make(chan string, capacity)

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers <- r.Header.Get(AuthHeader)

		w.WriteHeader(http.StatusOK)
	}))

	t.Cleanup(srv.Close)

	return srv, headers
}

// callerDeadline is short enough to prove a caller left on its own schedule rather than waiting
// out a fetch that never returns, and long enough to stay clear of scheduling jitter on a loaded
// CI runner.
const callerDeadline = 300 * time.Millisecond

// pileupBound is well under tokenRequestTimeout, so it distinguishes "the caller left on its own
// deadline" from "the caller queued behind a fetch attempt": one queued caller alone would
// already exceed it.
const pileupBound = 3 * time.Second

// hangingTokenServer is a token endpoint that accepts a request and never answers until released,
// so a test can hold a fetch in flight for exactly as long as it needs to.
type hangingTokenServer struct {
	*httptest.Server

	release chan struct{}
}

// newHangingTokenServer starts the server and arranges for release to be closed on test cleanup,
// so a request left hanging when the test ends does not block httptest.Server.Close.
func newHangingTokenServer(t *testing.T) *hangingTokenServer {
	t.Helper()

	ts := &hangingTokenServer{release: make(chan struct{})}

	ts.Server = httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-ts.release
	}))

	t.Cleanup(func() {
		close(ts.release)
		ts.Close()
	})

	return ts
}

func (ts *hangingTokenServer) oAuthConfig() *OAuthConfig {
	return &OAuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		TokenURL:     ts.URL,
		AuthStyle:    oauth2.AuthStyleInParams,
	}
}

// TestOAuth_TokenIsReusedAcrossRequests tests that a token granted with expires_in=3600 is reused
// for later calls instead of being minted again on every request.
func TestOAuth_TokenIsReusedAcrossRequests(t *testing.T) {
	const calls = 10

	tokenServer := newCountingTokenServer(t, tokenLifetime)
	upstream, seen := recordingUpstream(t, calls)

	svc := NewHTTPService(upstream.URL, logging.NewMockLogger(logging.INFO), nil, tokenServer.oAuthConfig())

	for i := range calls {
		resp, err := svc.Get(t.Context(), "test", nil)
		require.NoErrorf(t, err, "call %d", i)
		require.NoError(t, resp.Body.Close())
	}

	assert.Equal(t, int64(1), tokenServer.grants.Load(),
		"token endpoint should be called once for %d requests", calls)

	for range calls {
		assert.Equal(t, "Bearer token-1", <-seen)
	}
}

// TestOAuth_ExpiredTokenIsRefetched tests that caching does not serve a token past its lifetime.
func TestOAuth_ExpiredTokenIsRefetched(t *testing.T) {
	const calls = 3

	tokenServer := newCountingTokenServer(t, shortTokenLifetime)
	upstream, seen := recordingUpstream(t, calls)

	svc := NewHTTPService(upstream.URL, logging.NewMockLogger(logging.INFO), nil, tokenServer.oAuthConfig())

	for i := range calls {
		resp, err := svc.Get(t.Context(), "test", nil)
		require.NoErrorf(t, err, "call %d", i)
		require.NoError(t, resp.Body.Close())
	}

	assert.Equal(t, int64(calls), tokenServer.grants.Load(),
		"an expired token must be replaced, not reused")

	for i := range calls {
		assert.Equal(t, fmt.Sprintf("Bearer token-%d", i+1), <-seen)
	}
}

// TestOAuth_ConcurrentRequestsShareOneTokenGrant tests that a burst of callers arriving with no
// cached token produces a single grant rather than one per caller.
func TestOAuth_ConcurrentRequestsShareOneTokenGrant(t *testing.T) {
	const callers = 25

	tokenServer := newCountingTokenServer(t, tokenLifetime)
	provider := tokenServer.oAuthConfig().newTokenProvider()

	var wg sync.WaitGroup

	granted := make(chan string, callers)

	wg.Add(callers)

	for range callers {
		go func() {
			defer wg.Done()

			headers, err := provider.addAuthorizationHeader(t.Context(), nil)
			if err != nil {
				return
			}

			granted <- headers[AuthHeader]
		}()
	}

	wg.Wait()
	close(granted)

	assert.Equal(t, int64(1), tokenServer.grants.Load(), "concurrent callers should share one grant")
	assert.Len(t, granted, callers)

	for header := range granted {
		assert.Equal(t, "Bearer token-1", header)
	}
}

// TestOAuth_FailedTokenGrantIsNotCached tests that a failed grant is retried on the next call
// rather than being remembered for the process lifetime.
func TestOAuth_FailedTokenGrantIsNotCached(t *testing.T) {
	tokenServer := newCountingTokenServer(t, tokenLifetime)
	tokenServer.rejects.Store(1)

	provider := tokenServer.oAuthConfig().newTokenProvider()

	_, err := provider.addAuthorizationHeader(t.Context(), nil)
	require.Error(t, err, "a rejected grant must surface to the caller")

	headers, err := provider.addAuthorizationHeader(t.Context(), nil)
	require.NoError(t, err, "the next call must retry the grant")
	assert.Equal(t, "Bearer token-1", headers[AuthHeader])
	assert.Equal(t, int64(1), tokenServer.grants.Load())
}

// TestOAuth_CallerLeavesOnOwnCancelledContext tests that a caller whose context is already
// canceled does not wait for a token fetch: it returns ctx.Err() straight away.
func TestOAuth_CallerLeavesOnOwnCancelledContext(t *testing.T) {
	tokenServer := newHangingTokenServer(t)
	provider := tokenServer.oAuthConfig().newTokenProvider()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := provider.addAuthorizationHeader(ctx, nil)
	require.ErrorIs(t, err, context.Canceled)
}

// TestOAuth_CallerLeavesOnOwnDeadline tests that a caller does not wait past its own context
// deadline for a token fetch that never returns.
//
// Building the token source per request (rather than reusing it) also fixed the token-endpoint
// grant count, but it did so by binding the fetch to the caller's own context, so a caller
// waiting on oauth2.ReuseTokenSource's lock while the fetch ahead of it hung would queue for a
// full tokenRequestTimeout per caller in front of it, regardless of its own deadline. This pins the
// fix: token lets a caller leave on its own schedule.
func TestOAuth_CallerLeavesOnOwnDeadline(t *testing.T) {
	tokenServer := newHangingTokenServer(t)
	provider := tokenServer.oAuthConfig().newTokenProvider()

	ctx, cancel := context.WithTimeout(t.Context(), callerDeadline)
	defer cancel()

	start := time.Now()
	_, err := provider.addAuthorizationHeader(ctx, nil)
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, pileupBound, "caller should leave on its own deadline, not wait out the fetch")
}

// TestOAuth_ConcurrentCallersDoNotQueueBehindAHungFetch tests that callers arriving while a fetch
// is in flight do not serialize behind it: each still leaves on its own deadline instead of
// waiting its turn at the token source's lock behind every caller ahead of it.
func TestOAuth_ConcurrentCallersDoNotQueueBehindAHungFetch(t *testing.T) {
	const callers = 5

	tokenServer := newHangingTokenServer(t)
	provider := tokenServer.oAuthConfig().newTokenProvider()

	var wg sync.WaitGroup

	elapsed := make(chan time.Duration, callers)

	wg.Add(callers)

	for range callers {
		go func() {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(t.Context(), callerDeadline)
			defer cancel()

			start := time.Now()
			_, err := provider.addAuthorizationHeader(ctx, nil)

			assert.ErrorIs(t, err, context.DeadlineExceeded)

			elapsed <- time.Since(start)
		}()
	}

	wg.Wait()
	close(elapsed)

	for d := range elapsed {
		assert.Less(t, d, pileupBound, "no caller should queue behind another caller's fetch attempt")
	}
}

// TestOAuth_EachRegistrationGetsItsOwnTokenSource tests that the cache belongs to the registration
// AddOption creates, so two separately configured services do not share a token.
func TestOAuth_EachRegistrationGetsItsOwnTokenSource(t *testing.T) {
	tokenServer := newCountingTokenServer(t, tokenLifetime)
	upstream, seen := recordingUpstream(t, 2)

	for range 2 {
		svc := NewHTTPService(upstream.URL, logging.NewMockLogger(logging.INFO), nil, tokenServer.oAuthConfig())

		resp, err := svc.Get(t.Context(), "test", nil)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}

	assert.Equal(t, int64(2), tokenServer.grants.Load())
	assert.Equal(t, "Bearer token-1", <-seen)
	assert.Equal(t, "Bearer token-2", <-seen)
}
