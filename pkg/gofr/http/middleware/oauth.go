package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	errEmptyProvider       = errors.New("require non-empty provider")
	errInvalidInterval     = errors.New("invalid interval, require a value greater than 1 second")
	errEmptyModulus        = errors.New("modulus is empty")
	errEmptyPublicExponent = errors.New("public exponent is empty")
	errEmptyResponseBody   = errors.New("response body is empty")
	errInvalidURL          = errors.New("invalid URL")
)

const jwtRegexPattern = "^[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+$"

// PublicKeys stores a map of public keys identified by their key ID (kid).
type PublicKeys struct {
	mu   sync.RWMutex
	keys map[string]*rsa.PublicKey
}

// JWKNotFound is an error type indicating a missing JSON Web Key Set (JWKS).
type JWKNotFound struct {
}

func (JWKNotFound) Error() string {
	return "JWKS Not Found"
}

// Get retrieves a public key from the PublicKeys map by its key ID.
func (p *PublicKeys) Get(kid string) *rsa.PublicKey {
	p.mu.RLock()
	defer p.mu.RUnlock()
	key := p.keys[strings.TrimSpace(kid)]

	return key
}

type JWKSProvider interface {
	GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
		headers map[string]string) (*http.Response, error)
}

// OauthConfigs holds configuration for OAuth middleware.
type OauthConfigs struct {
	Provider        JWKSProvider
	RefreshInterval time.Duration
	Path            string
}

// NewOAuth creates a PublicKeyProvider that periodically fetches and updates public keys from a JWKS endpoint.
func NewOAuth(config OauthConfigs) PublicKeyProvider {
	var publicKeys PublicKeys

	go func() {
		publicKeys.updateKeys(config)

		ticker := time.NewTicker(config.RefreshInterval)
		defer ticker.Stop()

		for range ticker.C {
			publicKeys.updateKeys(config)
		}
	}()

	return &publicKeys
}

// updateKeys updates keys using PublicKeyProvider.
func (p *PublicKeys) updateKeys(config OauthConfigs) {
	jwks, err := getPublicKeys(context.Background(), config.Provider, config.Path)
	if err != nil {
		return
	}

	keys := publicKeyFromJWKS(jwks)
	if len(keys) == 0 {
		return
	}

	p.mu.Lock()
	p.keys = keys
	p.mu.Unlock()
}

// getPublicKeys fetches the public keys from JWKSProvider and returns JWKS.
func getPublicKeys(ctx context.Context, provider JWKSProvider, path string) (JWKS, error) {
	var keys JWKS

	resp, err := provider.GetWithHeaders(ctx, path, nil, nil)
	if err != nil || resp == nil {
		return keys, err
	}

	if resp.StatusCode != http.StatusOK {
		return keys, errInvalidURL
	}

	if resp.Body == nil {
		return keys, errEmptyResponseBody
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return keys, err
	}

	resp.Body.Close()

	err = json.Unmarshal(body, &keys)

	return keys, err
}

// PublicKeyProvider defines an interface for retrieving a public key by its key ID.
type PublicKeyProvider interface {
	Get(kid string) *rsa.PublicKey
}

// OAuth is a middleware function that validates JWT access tokens using a provided PublicKeyProvider.
func OAuth(key PublicKeyProvider, options ...jwt.ParserOption) func(http.Handler) http.Handler {
	// error being ignored is not the right behavior, this function should be deprecated and use NewOAuthProvider() instead.
	function, _ := getPublicKeyFunc(key)
	provider := OAuthProvider{
		publicKeyFunc: function,
		options:       append(options, jwt.WithIssuedAt()),
		regex:         regexp.MustCompile(jwtRegexPattern),
	}

	return AuthMiddleware(&provider)
}

// JWKS represents a JSON Web Key Set.
type JWKS struct {
	Keys []JSONWebKey `json:"keys"`
}

// JSONWebKey represents a JSON Web Key.
type JSONWebKey struct {
	ID   string `json:"kid"`
	Type string `json:"kty"`

	Modulus         string `json:"n"`
	PublicExponent  string `json:"e"`
	PrivateExponent string `json:"d"`
}

// publicKeyFromJWKS creates a public key from a JWKS and returns it in string .
func publicKeyFromJWKS(jwks JWKS) map[string]*rsa.PublicKey {
	if len(jwks.Keys) == 0 {
		return nil
	}

	keys := make(map[string]*rsa.PublicKey)

	for _, jwk := range jwks.Keys {
		if key, err := jwk.rsaPublicKey(); err == nil {
			keys[jwk.ID] = key
		}
	}

	return keys
}

// rsaPublicKey returns the rsa.PublicKey value for JSONWebKey.
func (jwk *JSONWebKey) rsaPublicKey() (*rsa.PublicKey, error) {
	if jwk.Modulus == "" {
		return nil, errEmptyModulus
	}

	if jwk.PublicExponent == "" {
		return nil, errEmptyPublicExponent
	}

	n, err := base64.RawURLEncoding.DecodeString(jwk.Modulus)
	if err != nil {
		return nil, err
	}

	e, err := base64.RawURLEncoding.DecodeString(jwk.PublicExponent)
	if err != nil {
		return nil, err
	}

	nInt := new(big.Int).SetBytes(n)
	eInt := new(big.Int).SetBytes(e)

	rsaPublicKey := &rsa.PublicKey{
		N: nInt,
		E: int(eInt.Int64()),
	}

	return rsaPublicKey, nil
}

type OAuthProvider struct {
	publicKeyFunc func(token *jwt.Token) (any, error)
	// keyProvider PublicKeyProvider
	options []jwt.ParserOption
	regex   *regexp.Regexp
}

// NewOAuthProvider generates a OAuthProvider for the given OauthConfigs and jwt.ParserOption.
func NewOAuthProvider(config OauthConfigs, options ...jwt.ParserOption) (AuthProvider, error) {
	function, err := getPublicKeyFunc(NewOAuth(config))
	if err != nil {
		return nil, err
	}

	if config.RefreshInterval <= time.Second {
		return nil, errInvalidInterval
	}

	return &OAuthProvider{
		publicKeyFunc: function,
		regex:         regexp.MustCompile(jwtRegexPattern),
		options:       append(options, jwt.WithIssuedAt()),
	}, nil
}

func (p *OAuthProvider) ExtractAuthHeader(r *http.Request) (any, ErrorHTTP) {
	header, err := getAuthHeaderFromRequest(r, headerAuthorization, "Bearer")
	if err != nil {
		return nil, err
	}

	if !p.regex.MatchString(header) {
		return nil, NewInvalidAuthorizationHeaderFormatError(headerAuthorization, "jwt expected")
	}

	token, parseErr := jwt.Parse(header, p.publicKeyFunc, p.options...)

	if parseErr != nil {
		if strings.Contains(parseErr.Error(), "token is malformed") {
			return nil, NewInvalidAuthorizationHeaderFormatError(headerAuthorization, "token is malformed")
		}

		if errors.Is(parseErr, errEmptyProvider) {
			return nil, NewInvalidConfigurationError("jwks configuration issue")
		}

		return nil, NewInvalidAuthorizationHeaderError(headerAuthorization)
	}

	// Verify if this typecasting is really required, it may be unnecessary
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, NewInvalidAuthorizationHeaderFormatError(headerAuthorization, jwt.ErrTokenInvalidClaims.Error())
	}

	return claims, nil
}

// GetAuthMethod returns JWTClaim authMethod.
func (*OAuthProvider) GetAuthMethod() AuthMethod {
	return JWTClaim
}

// getPublicKeyFunc returns keyFunc to be used in jwt.Parse().
// In case given PublicKeyProvider is nil, nil keyFunc is returned along with errEmptyProvider error.
func getPublicKeyFunc(provider PublicKeyProvider) (func(token *jwt.Token) (any, error), error) {
	if provider == nil {
		return nil, errEmptyProvider
	}

	return func(token *jwt.Token) (any, error) {
		kid := token.Header["kid"]
		jwks := provider.Get(fmt.Sprint(kid))

		if jwks == nil {
			return nil, JWKNotFound{}
		}

		return jwks, nil
	}, nil
}
