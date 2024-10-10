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
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	errAuthorizationHeaderRequired = errors.New("authorization header is required")
	errInvalidAuthorizationHeader  = errors.New("authorization header format must be Bearer {token}")
)

// authMethod represents a custom type to define the different authentication methods supported.
type authMethod int

const (
	JWTClaim authMethod = iota // JWTClaim represents the key used to store JWT claims within the request context.
)

// PublicKeys stores a map of public keys identified by their key ID (kid).
type PublicKeys struct {
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
	kid = strings.TrimSpace(kid)

	return p.keys[kid]
}

type JWKSProvider interface {
	GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
		headers map[string]string) (*http.Response, error)
}

// OauthConfigs holds configuration for OAuth middleware.
type OauthConfigs struct {
	Provider        JWKSProvider
	RefreshInterval time.Duration
}

// NewOAuth creates a PublicKeyProvider that periodically fetches and updates public keys from a JWKS endpoint.
func NewOAuth(config OauthConfigs) PublicKeyProvider {
	var publicKeys PublicKeys

	go func() {
		ticker := time.NewTicker(config.RefreshInterval)
		defer ticker.Stop()

		for range ticker.C {
			keys, err := updateKeys(config)
			if err != nil || keys == nil {
				continue
			}

			publicKeys = *keys
		}
	}()

	return &publicKeys
}

func updateKeys(config OauthConfigs) (*PublicKeys, error) {
	resp, err := config.Provider.GetWithHeaders(context.Background(), "", nil, nil)
	if err != nil || resp == nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	resp.Body.Close()

	var keys JWKS

	err = json.Unmarshal(body, &keys)
	if err != nil {
		return nil, err
	}

	var publicKeys PublicKeys
	publicKeys.keys = make(map[string]*rsa.PublicKey)

	publicKeys.keys = publicKeyFromJWKS(keys)

	return &publicKeys, nil
}

// PublicKeyProvider defines an interface for retrieving a public key by its key ID.
type PublicKeyProvider interface {
	Get(kid string) *rsa.PublicKey
}

// OAuth is a middleware function that validates JWT access tokens using a provided PublicKeyProvider.
func OAuth(key PublicKeyProvider) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isWellKnown(r.URL.Path) {
				inner.ServeHTTP(w, r)
				return
			}

			tokenString, err := extractToken(r.Header.Get("Authorization"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			token, err := parseToken(tokenString, key)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), JWTClaim, token.Claims)
			*r = *r.Clone(ctx)

			inner.ServeHTTP(w, r)
		})
	}
}

// ExtractToken validates the Authorization header and extracts the JWT token.
func extractToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errAuthorizationHeaderRequired
	}

	const bearerPrefix = "Bearer "

	token, ok := strings.CutPrefix(authHeader, bearerPrefix)
	if !ok || token == "" {
		return "", errInvalidAuthorizationHeader
	}

	return token, nil
}

// ParseToken parses the JWT token using the provided key provider.
func parseToken(tokenString string, key PublicKeyProvider) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		kid := token.Header["kid"]
		jwks := key.Get(fmt.Sprint(kid))

		if jwks == nil {
			return nil, JWKNotFound{}
		}

		return jwks, nil
	})
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

// PublicKeyFromJWKS creates a public key from a JWKS and returns it in string format.
func publicKeyFromJWKS(jwks JWKS) map[string]*rsa.PublicKey {
	if len(jwks.Keys) == 0 {
		return nil
	}

	keys := make(map[string]*rsa.PublicKey)

	for _, jwk := range jwks.Keys {
		var val = jwk

		keys[jwk.ID], _ = rsaPublicKeyStringFromJWK(&val)
	}

	return keys
}

func rsaPublicKeyStringFromJWK(jwk *JSONWebKey) (*rsa.PublicKey, error) {
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
