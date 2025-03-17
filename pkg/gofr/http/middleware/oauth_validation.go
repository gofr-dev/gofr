package middleware

import (
	"github.com/golang-jwt/jwt/v5"
	"time"
)

func validateClaims(claims jwt.MapClaims, config *ClaimConfig) error {
	validators := []func(jwt.MapClaims, *ClaimConfig) error{
		validateIssuer,
		validateAudience,
		validateSubject,
		validateExpiry,
		validateNotBefore,
		validateIssuedAt,
		validateJTI,
		validateRoles,
	}

	for _, validate := range validators {
		if err := validate(claims, config); err != nil {
			return err
		}
	}
	return nil
}

func validateIssuer(claims jwt.MapClaims, config *ClaimConfig) error {
	if len(config.TrustedIssuers) == 0 {
		return nil
	}

	iss, ok := claims["iss"].(string)
	if !ok || !contains(config.TrustedIssuers, iss) {
		return errInvalidIssuer
	}

	return nil
}

func validateAudience(claims jwt.MapClaims, config *ClaimConfig) error {
	if len(config.ValidAudiences) == 0 {
		return nil
	}

	switch aud := claims["aud"].(type) {
	case string:
		if !contains(config.ValidAudiences, aud) {
			return errInvalidAudience
		}
	case []any:
		if !containsAny(config.ValidAudiences, aud) {
			return errInvalidAudience
		}
	default:
		return errInvalidAudience
	}

	return nil
}

func validateSubject(claims jwt.MapClaims, config *ClaimConfig) error {
	if len(config.AllowedSubjects) == 0 {
		return nil
	}

	sub, ok := claims["sub"].(string)
	if !ok || !contains(config.AllowedSubjects, sub) {
		return errInvalidSubject
	}

	return nil
}

func validateExpiry(claims jwt.MapClaims, config *ClaimConfig) error {
	if !config.CheckExpiry {
		return nil
	}

	exp, ok := claims["exp"].(float64)
	if !ok || time.Now().Unix() > int64(exp) {
		return errTokenExpired
	}

	return nil
}

func validateNotBefore(claims jwt.MapClaims, config *ClaimConfig) error {
	if !config.CheckNotBefore {
		return nil
	}

	nbf, ok := claims["nbf"].(float64)
	if ok && time.Now().Unix() < int64(nbf) {
		return errTokenNotActive
	}

	return nil
}

func validateIssuedAt(claims jwt.MapClaims, config *ClaimConfig) error {
	if !config.CheckIssuedAt {
		return nil
	}

	iat, ok := claims["iat"].(float64)
	if !ok || time.Now().Unix() < int64(iat) {
		return errInvalidIssuedAt
	}

	return nil
}

func validateJTI(claims jwt.MapClaims, config *ClaimConfig) error {
	if config.ValidateJTI == nil {
		return nil
	}

	jti, ok := claims["jti"].(string)
	if !ok || !config.ValidateJTI(jti) {
		return errInvalidJTI
	}

	return nil
}

func validateRoles(claims jwt.MapClaims, config *ClaimConfig) error {
	if len(config.RequiredRoles) == 0 {
		return nil
	}

	roles, _ := claims["roles"].([]any)
	if !hasRequiredRole(roles, config.RequiredRoles) {
		return errInvalidRole
	}

	return nil
}

func hasRequiredRole(roles []any, required []string) bool {
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

func containsAny(s []string, items []any) bool {
	for _, item := range items {
		if str, ok := item.(string); ok && contains(s, str) {
			return true
		}
	}

	return false
}
