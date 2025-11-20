package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// MockJWKSServer provides a mock JWKS endpoint for testing
type MockJWKSServer struct {
	server     *httptest.Server
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	keyID      string
}

// NewMockJWKSServer creates a new mock JWKS server with RSA keys
func NewMockJWKSServer() (*MockJWKSServer, error) {
	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	publicKey := &privateKey.PublicKey
	keyID := "test-key-id"

	// Create JWKS response
	jwks := createJWKS(publicKey, keyID)

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	})

	server := httptest.NewServer(mux)

	return &MockJWKSServer{
		server:     server,
		privateKey: privateKey,
		publicKey:  publicKey,
		keyID:      keyID,
	}, nil
}

// URL returns the base URL of the mock server
func (m *MockJWKSServer) URL() string {
	return m.server.URL
}

// JWKSEndpoint returns the full JWKS endpoint URL
func (m *MockJWKSServer) JWKSEndpoint() string {
	return m.server.URL + "/.well-known/jwks.json"
}

// GenerateToken creates a JWT token with the given claims, signed with the server's private key
func (m *MockJWKSServer) GenerateToken(claims jwt.MapClaims) (string, error) {
	// Set standard claims if not present
	if claims["iat"] == nil {
		claims["iat"] = time.Now().Unix()
	}
	if claims["exp"] == nil {
		claims["exp"] = time.Now().Add(time.Hour).Unix()
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = m.keyID

	return token.SignedString(m.privateKey)
}

// Close shuts down the mock server
func (m *MockJWKSServer) Close() {
	m.server.Close()
}

// createJWKS creates a JWKS response from an RSA public key
func createJWKS(publicKey *rsa.PublicKey, keyID string) map[string]interface{} {
	// Encode modulus (n) and exponent (e) to base64url
	nBytes := publicKey.N.Bytes()
	eBytes := big.NewInt(int64(publicKey.E)).Bytes()

	nBase64 := base64.RawURLEncoding.EncodeToString(nBytes)
	eBase64 := base64.RawURLEncoding.EncodeToString(eBytes)

	return map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"kid": keyID,
				"n":   nBase64,
				"e":   eBase64,
				"use": "sig",
				"alg": "RS256",
			},
		},
	}
}

