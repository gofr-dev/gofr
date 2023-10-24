package oauth

// JWK - JSON WEB KEY
// JWKs - JSON WEB KEY SET

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"time"

	"gofr.dev/pkg/middleware"

	"gofr.dev/pkg/log"
)

// getPublicKey returns a JWK based public key for the given KID
func (o *OAuth) loadJWK(logger log.Logger) ([]PublicKey, error) {
	// if key is not present in memory get it from endpoint
	resp, err := http.Get(o.options.JWKPath)

	if err != nil {
		logger.Errorf("Failed to fetch the public key from the specified url. Got error : %v", err)
		return nil, middleware.ErrServiceDown
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("Failed to fetch the response body. Got error : %v", err)
		return nil, middleware.ErrServiceDown
	}

	var k PublicKeys

	err = json.Unmarshal(body, &k)
	if err != nil {
		logger.Errorf("Failed to unmarshal the json response body. Got error : %v", err)
		return nil, middleware.ErrServiceDown
	}

	return k.Keys, nil
}

//nolint:revive //in accordance with RFC specification
func (publicKeys *PublicKeys) Get(kID string) *PublicKey {
	for k := range publicKeys.Keys {
		key := publicKeys.Keys[k]
		if key.ID == kID {
			return &key
		}
	}

	return &PublicKey{}
}

func (key *PublicKey) getRSAPublicKey() (rsa.PublicKey, error) {
	if key.rsaPublicKey.N != nil {
		return key.rsaPublicKey, nil
	}

	rsaPublicKey, err := generateRSAPublicKey(key)
	if err != nil {
		return key.rsaPublicKey, err
	}

	key.rsaPublicKey = rsaPublicKey

	return key.rsaPublicKey, nil
}

// generateRSAPublicKey takes public key as type PublicKey with
// field values as string. The modulus(n) and exponent(e) are parsed
// into a struct of type rsa.PublicKey.
func generateRSAPublicKey(key *PublicKey) (rsa.PublicKey, error) {
	var publicKey rsa.PublicKey

	// MODULUS
	decN, err := base64.RawURLEncoding.DecodeString(key.Modulus)
	if err != nil {
		return publicKey, err
	}

	n := big.NewInt(0)
	n.SetBytes(decN)

	// EXPONENT
	decE, err := base64.RawURLEncoding.DecodeString(key.PublicExponent)
	if err != nil {
		return publicKey, err
	}

	var eBytes []byte

	const DecStrLen = 8
	if len(decE) < DecStrLen {
		eBytes = make([]byte, DecStrLen-len(decE), DecStrLen)
		eBytes = append(eBytes, decE...)
	} else {
		eBytes = decE
	}

	eReader := bytes.NewReader(eBytes)

	var e uint64

	err = binary.Read(eReader, binary.BigEndian, &e)
	if err != nil {
		return publicKey, err
	}

	publicKey.N = n
	publicKey.E = int(e)

	return publicKey, nil
}

func (o *OAuth) invalidateCache(logger log.Logger) error {
	var err error

	o.cache.mu.Lock()
	var keys []PublicKey

	duration := o.options.ValidityFrequency
	if keys, err = o.loadJWK(logger); err != nil {
		duration = 3
	} else {
		// save the public keys in memory
		o.cache.publicKeys.Keys = keys
	}

	if duration > 0 {
		go func() {
			time.Sleep(time.Duration(duration) * time.Second)

			_ = o.invalidateCache(logger)
		}()
	}
	o.cache.mu.Unlock()

	return err
}
