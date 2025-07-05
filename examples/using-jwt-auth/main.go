package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
	"gofr.dev/examples/using-jwt-auth/handler"
	"gofr.dev/pkg/gofr"
)

var (
	rsaPrivateKey *rsa.PrivateKey
	rsaPublicKey  *rsa.PublicKey
)

func init() {
	var err error

	// ✅ Read from env
	privateKeyPath := os.Getenv("PRIVATE_KEY_PATH")
	if privateKeyPath == "" {
		log.Fatal("Environment variable PRIVATE_KEY_PATH is not set")
	}

	publicKeyPath := os.Getenv("PUBLIC_KEY_PATH")
	if publicKeyPath == "" {
		log.Fatal("Environment variable PUBLIC_KEY_PATH is not set")
	}

	rsaPrivateKey, err = loadRSAPrivateKey(privateKeyPath)
	if err != nil {
		log.Fatalf("Failed to load RSA private key: %v", err)
	}

	rsaPublicKey, err = loadRSAPublicKey(publicKeyPath)
	if err != nil {
		log.Fatalf("Failed to load RSA public key: %v", err)
	}
}

func loadRSAPrivateKey(filePath string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading private key file failed: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("invalid or missing PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return key, nil
	}

	// Try PKCS8 fallback
	if keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := keyAny.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
	}

	return nil, fmt.Errorf("unable to parse RSA private key")
}

func loadRSAPublicKey(filePath string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading public key file failed: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("invalid or missing PEM block")
	}

	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse public key: %w", err)
	}

	pubKey, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return pubKey, nil
}

func JWKSHandler(ctx *gofr.Context) (interface{}, error) {
	if rsaPublicKey == nil {
		return nil, CustomInternalServerError{"Public key not available"}
	}

	jwks := map[string]interface{}{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"use": "sig",
				"kid": "my-rsa-key",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(rsaPublicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaPublicKey.E)).Bytes()),
			},
		},
	}
	return jwks, nil
}

type CustomInternalServerError struct {
	Message string
}

func (e CustomInternalServerError) Error() string {
	return e.Message
}

func (e CustomInternalServerError) StatusCode() int {
	return http.StatusInternalServerError
}

func main() {
	app := gofr.New()

	// POST /login issues RS256-signed token
	app.POST("/login", handler.LoginHandler(rsaPrivateKey))

	// JWKS endpoint to serve public key
	app.GET("/oauth2/jwks", JWKSHandler)

	// Enable OAuth validation using public JWKS
	app.EnableOAuth(
		"http://localhost:8000/oauth2/jwks", // If you change port, update this
		60,
		jwt.WithExpirationRequired(),
	)

	// Secured route
	app.GET("/secure", handler.SecureHandler)

	app.Run()
}