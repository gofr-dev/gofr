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
	errInvalidIssuer               = errors.New("invalid issuer")
	errInvalidAudience             = errors.New("invalid audience")
	errTokenExpired                = errors.New("token has expired")
	errTokenNotActive              = errors.New("token is not active yet")
	errInvalidRole                 = errors.New("insufficient permissions")
	errInvalidSubject              = errors.New("invalid subject")
	errInvalidIssuedAt             = errors.New("invalid issued at time")
	errInvalidJTI                  = errors.New("invalid JWT ID")
)

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
	GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
		headers map[string]string) (*http.Response, error)
}

// OauthConfigs holds configuration for OAuth middleware.
type OauthConfigs struct {
	Provider        JWKSProvider
	RefreshInterval time.Duration
}

type ClaimOption func(*ClaimConfig)

type ClaimConfig struct {
	RequiredRoles   []string
	TrustedIssuers  []string
	ValidAudiences  []string
	AllowedSubjects []string
	CheckExpiry     bool
	CheckNotBefore  bool
	CheckIssuedAt   bool
	ValidateJTI     func(string) bool
}

func WithRequiredRoles(roles ...string) ClaimOption {
	return func(cfg *ClaimConfig) {
		cfg.RequiredRoles = roles
	}
}

func WithTrustedIssuers(issuers ...string) ClaimOption {
	return func(cfg *ClaimConfig) {
		cfg.TrustedIssuers = issuers
	}
}

func WithValidAudiences(audiences ...string) ClaimOption {
	return func(cfg *ClaimConfig) {
		cfg.ValidAudiences = audiences
	}
}

func WithAllowedSubjects(subjects ...string) ClaimOption {
	return func(cfg *ClaimConfig) {
		cfg.AllowedSubjects = subjects
	}
}

func WithCheckExpiry() ClaimOption {
	return func(cfg *ClaimConfig) {
		cfg.CheckExpiry = true
	}
}

func WithCheckNotBefore() ClaimOption {
	return func(cfg *ClaimConfig) {
		cfg.CheckNotBefore = true
	}
}

func WithCheckIssuedAt() ClaimOption {
	return func(cfg *ClaimConfig) {
		cfg.CheckIssuedAt = true
	}
}

func WithJTIValidator(fn func(string) bool) ClaimOption {
	return func(cfg *ClaimConfig) {
		cfg.ValidateJTI = fn
	}
}

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
func OAuth(key PublicKeyProvider, config *ClaimConfig) func(inner http.Handler) http.Handler {
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

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "invalid claims", http.StatusUnauthorized)
				return
			}

			if err := validateClaims(claims, config); err != nil {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), JWTClaim, token.Claims)
			*r = *r.Clone(ctx)

			inner.ServeHTTP(w, r)
		})
	}
}

func validateClaims(claims jwt.MapClaims, config *ClaimConfig) error {
	if len(config.TrustedIssuers) > 0 {
		iss, ok := claims["iss"].(string)
		if !ok || !contains(config.TrustedIssuers, iss) {
			return errInvalidIssuer
		}
	}

	if len(config.ValidAudiences) > 0 {
		switch aud := claims["aud"].(type) {
		case string:
			if !contains(config.ValidAudiences, aud) {
				return errInvalidAudience
			}
		case []interface{}:
			if !containsAny(config.ValidAudiences, aud) {
				return errInvalidAudience
			}
		default:
			return errInvalidAudience
		}
	}

	if len(config.AllowedSubjects) > 0 {
		sub, ok := claims["sub"].(string)
		if !ok || !contains(config.AllowedSubjects, sub) {
			return errInvalidSubject
		}
	}

	if config.CheckExpiry {
		exp, ok := claims["exp"].(float64)
		if !ok || time.Now().Unix() > int64(exp) {
			return errTokenExpired
		}
	}

	if config.CheckNotBefore {
		nbf, ok := claims["nbf"].(float64)
		if ok && time.Now().Unix() < int64(nbf) {
			return errTokenNotActive
		}
	}

	if config.CheckIssuedAt {
		iat, ok := claims["iat"].(float64)
		if !ok || time.Now().Unix() < int64(iat) {
			return errInvalidIssuedAt
		}
	}

	if config.ValidateJTI != nil {
		jti, ok := claims["jti"].(string)
		if !ok || !config.ValidateJTI(jti) {
			return errInvalidJTI
		}
	}

	if len(config.RequiredRoles) > 0 {
		roles, _ := claims["roles"].([]interface{})
		if !hasRequiredRole(roles, config.RequiredRoles) {
			return errInvalidRole
		}
	}

	return nil
}

func hasRequiredRole(roles []interface{}, required []string) bool {
	for _, r := range required {
		for _, role := range roles {
			if roleStr, ok := role.(string); ok && roleStr == r {
				return true
			}
		}
	}

	return false
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func containsAny(s []string, items []interface{}) bool {
	for _, item := range items {
		if str, ok := item.(string); ok && contains(s, str) {
			return true
		}
	}

	return false
}

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
	return jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
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
