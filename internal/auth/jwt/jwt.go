package jwt

import (
	// Standard library.
	"errors"
	"fmt"
	"log"
	"time"

	// Third-party.
	"github.com/golang-jwt/jwt/v4"
)

var (
	ErrTokenSigningEmptyKey    = errors.New("token signing error: no secret configured")
	ErrUnexpectedSigningMethod = errors.New("unexpected signing method")
	ErrInvalidToken            = errors.New("invalid token")
	ErrInvalidClaims           = errors.New("invalid claims")
	ErrMissingSubClaim         = errors.New("missing sub claim")
)

// User interface to avoid circular import.
type User interface {
	GetDN() string
	GetUsername() string
	GetEmail() string
	GetFullName() string
	GetGroups() []string
	GetDepartment() string
}

// Issuer implements TokenIssuer interface for JWT tokens.
type Issuer struct {
	secret          string
	ttl             time.Duration
	includeUserInfo bool
}

// NewIssuer creates a new JWT token issuer.
func NewIssuer(secret string, ttl time.Duration, includeUserInfo bool) *Issuer {
	return &Issuer{
		secret:          secret,
		ttl:             ttl,
		includeUserInfo: includeUserInfo,
	}
}

// IssueToken creates and signs a JWT token for the given user.
func (j *Issuer) IssueToken(user User) (string, error) {
	if j.secret == "" {
		return "", ErrTokenSigningEmptyKey
	}

	claims := jwt.MapClaims{
		"sub": user.GetUsername(),
		"exp": time.Now().Add(j.ttl).Unix(),
		"iat": time.Now().Unix(),
	}

	// Optionally include user information in token claims.
	if j.includeUserInfo {
		if email := user.GetEmail(); email != "" {
			claims["email"] = email
		}

		if fullName := user.GetFullName(); fullName != "" {
			claims["name"] = fullName
		}

		if dept := user.GetDepartment(); dept != "" {
			claims["department"] = dept
		}

		if groups := user.GetGroups(); len(groups) > 0 {
			claims["groups"] = groups
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString([]byte(j.secret))
	if err != nil {
		log.Printf("token signing error: %v", err)
		return "", ErrTokenSigningEmptyKey
	}

	return signed, nil
}

// ValidateToken parses and validates a JWT, returning the 'sub' claim.
func (j *Issuer) ValidateToken(tokenStr string) (string, error) {
	keyFn := func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			log.Printf("unexpected signing method: %v", token.Header["alg"])
			return nil, ErrUnexpectedSigningMethod
		}

		secret := []byte(j.secret)

		return secret, nil
	}

	token, err := jwt.Parse(tokenStr, keyFn)
	if err != nil {
		return "", fmt.Errorf("token parse error: %w", err)
	}

	if !token.Valid {
		return "", ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", ErrInvalidClaims
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return "", ErrMissingSubClaim
	}

	return sub, nil
}
