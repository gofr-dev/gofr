package middleware

import (
	"fmt"
	"slices"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func validateClaims(claims jwt.MapClaims, config *ClaimConfig) error {
	validators := []func(jwt.MapClaims, *ClaimConfig) error{
		validateIssuer,
		validateAudience,
		validateSubject,
		validateExpiry,
		validateNotBefore,
		validateIssuedAt,
	}

	for _, validate := range validators {
		if err := validate(claims, config); err != nil {
			return err
		}
	}

	return nil
}

func validateIssuer(claims jwt.MapClaims, config *ClaimConfig) error {
	if config.trustedIssuer == "" {
		return nil
	}

	iss, ok := claims["iss"].(string)
	if !ok || iss != config.trustedIssuer {
		return errInvalidIssuer
	}
	return nil
}

func validateAudience(claims jwt.MapClaims, config *ClaimConfig) error {
	if len(config.validAudiences) == 0 {
		return nil
	}

	audClaim, exists := claims["aud"]
	if !exists {
		return errInvalidAudience
	}

	switch aud := audClaim.(type) {
	case string:
		if !slices.Contains(config.validAudiences, aud) {
			return errInvalidAudience
		}
	case []interface{}:
		found := false
		for _, a := range aud {
			if aStr, ok := a.(string); ok && slices.Contains(config.validAudiences, aStr) {
				found = true
				break
			}
		}
		if !found {
			return errInvalidAudience
		}
	default:
		return errInvalidAudience
	}
	return nil
}

func validateSubject(claims jwt.MapClaims, config *ClaimConfig) error {
	if len(config.allowedSubjects) == 0 {
		return nil
	}

	subClaim, exists := claims["sub"]
	if !exists {
		return errInvalidSubjects
	}

	switch sub := subClaim.(type) {
	case string:
		if !slices.Contains(config.allowedSubjects, sub) {
			return errInvalidSubjects
		}
	case []interface{}:
		found := false
		for _, a := range sub {
			if aStr, ok := a.(string); ok && slices.Contains(config.allowedSubjects, aStr) {
				found = true
				break
			}
		}
		if !found {
			return errInvalidSubjects
		}
	default:
		return errInvalidSubjects
	}
	return nil
}

func validateExpiry(claims jwt.MapClaims, config *ClaimConfig) error {
	if !config.checkExpiry {
		return nil
	}

	exp, ok := claims["exp"].(float64)
	if !ok || time.Now().Unix() > int64(exp) {
		return errTokenExpired
	}

	return nil
}

func validateNotBefore(claims jwt.MapClaims, config *ClaimConfig) error {
	if !config.checkNotBefore {
		return nil
	}

	nbf, ok := claims["nbf"].(float64)
	if ok && time.Now().Unix() < int64(nbf) {
		return errTokenNotActive
	}

	return nil
}

func validateIssuedAt(claims jwt.MapClaims, config *ClaimConfig) error {
	if config == nil || !config.issuedAtRule.enabled {
		return nil
	}

	iatFloat, ok := claims["iat"].(float64)
	if !ok {
		return errInvalidIssuedAt
	}

	issuedAt := time.Unix(int64(iatFloat), 0)
	rule := config.issuedAtRule

	// Check all constraints in order of strictness
	if rule.exact != nil {
		if !issuedAt.Equal(*rule.exact) {
			return fmt.Errorf("token issued at %v does not match exact required time %v",
				issuedAt.Format(time.RFC3339), rule.exact.Format(time.RFC3339))
		}
		return nil
	}

	if rule.before != nil && !issuedAt.Before(*rule.before) {
		return fmt.Errorf("token issued at %v is not before %v",
			issuedAt.Format(time.RFC3339), rule.before.Format(time.RFC3339))
	}

	if rule.after != nil && !issuedAt.After(*rule.after) {
		return fmt.Errorf("token issued at %v is not after %v",
			issuedAt.Format(time.RFC3339), rule.after.Format(time.RFC3339))
	}

	return nil
}
