package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaim string

type PublicKeys struct {
	keys map[string]*rsa.PublicKey
}

type JWKNotFound struct {
}

func (i JWKNotFound) Error() string {
	return "JWKS Not Found"
}

func (p *PublicKeys) Get(kid string) *rsa.PublicKey {
	kid = strings.TrimSpace(kid)

	return p.keys[kid]
}

type JWKSProvider interface {
	GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
		headers map[string]string) (*http.Response, error)
}

type OauthConfigs struct {
	Provider        JWKSProvider
	RefreshInterval time.Duration
}

func NewOAuth(config OauthConfigs) PublicKeyProvider {
	var publicKeys PublicKeys

	publicKeys.keys = make(map[string]*rsa.PublicKey)

	go func() {
		for {
			resp, err := config.Provider.GetWithHeaders(context.Background(), "", nil, nil)
			if err != nil || resp == nil {
				continue
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				continue
			}

			resp.Body.Close()

			var jwks JWKS

			err = json.Unmarshal(body, &jwks)
			if err != nil {
				continue
			}

			publicKeys.keys = publicKeyFromJWKS(jwks)

			time.Sleep(config.RefreshInterval)
		}
	}()

	return &publicKeys
}

type PublicKeyProvider interface {
	Get(kid string) *rsa.PublicKey
}

func OAuth(key PublicKeyProvider) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header is required", http.StatusUnauthorized)
				return
			}

			headerParts := strings.Split(authHeader, " ")
			if len(headerParts) != 2 || headerParts[0] != "Bearer" {
				http.Error(w, "Authorization header format must be Bearer {token}", http.StatusUnauthorized)
				return
			}

			tokenString := headerParts[1]

			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				kid := token.Header["kid"]

				jwks := key.Get(fmt.Sprint(kid))
				if jwks == nil {
					return nil, JWKNotFound{}
				}

				return key.Get(fmt.Sprint(kid)), nil
			})

			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(err.Error()))

				return
			}

			ctx := context.WithValue(r.Context(), JWTClaim("JWTClaims"), token.Claims)
			*r = *r.Clone(ctx)

			inner.ServeHTTP(w, r)
		})
	}
}

// JWKS represents a JSON Web Key Set.
type JWKS struct {
	Keys []JSONWebKey `json:"keys"`
}

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
