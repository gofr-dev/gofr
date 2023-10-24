package oauth

import (
	"crypto/rsa"
	"sync"
)

// OAuth struct manages OAuth options and caches public keys for JWT validation.
type OAuth struct {
	options Options
	cache   PublicKeyCache
}

// Options defines the validity frequency and JWK path for OAuth authentication.
type Options struct {
	// Set validity frequency in seconds
	ValidityFrequency int
	JWKPath           string
}

type header struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	URL       string `json:"jku"`
	KeyID     string `json:"kid"`
}

// JWT represents a JWT token, including its payload, header, and signature.
type JWT struct {
	payload   string
	header    header
	signature string
	token     string
}

// PublicKey encapsulates public key information used for JWT signature validation, including RSA fields.
type PublicKey struct {
	ID         string   `json:"kid"`
	Alg        string   `json:"alg"`
	Type       string   `json:"kty"`
	Use        string   `json:"use"`
	Operations []string `json:"key_ops"`

	// rsa fields
	Modulus         string `json:"n"`
	PublicExponent  string `json:"e"`
	PrivateExponent string `json:"d"`

	rsaPublicKey rsa.PublicKey
}

// PublicKeys holds a collection of public keys.
type PublicKeys struct {
	Keys []PublicKey `json:"keys"`
}

// PublicKeyCache caches public keys for JWT validation and manages concurrency.
type PublicKeyCache struct {
	publicKeys PublicKeys
	mu         sync.RWMutex
}
