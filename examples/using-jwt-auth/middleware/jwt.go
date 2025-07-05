package middleware

import (
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http/response"
	"github.com/golang-jwt/jwt/v5"
)

// JWTClaim is the key used to store JWT claims in the Go context.
// This is used by GetJWTClaims and potentially by GoFr's internal auth system.
// IMPORTANT: While SecureHandler uses ctx.GetAuthInfo().GetClaims(),
// this custom LoginHandler still generates tokens with specific claims,
// and these helpers are for accessing those if needed in other parts of your app
// that might store them with these keys (though GetAuthInfo is preferred for validated claims).
type contextKey string // Define a type for context keys to avoid collisions

const (
	JWTClaim   contextKey = "jwt_claims"
	UserIDKey  contextKey = "user_id"
	UsernameKey contextKey = "username"
	EmailKey   contextKey = "email"
	RolesKey   contextKey = "roles"
)

// CreateErrorResponse creates a structured error response with auth metadata.
// This function is used by LoginHandler and SecureHandler for consistent error formats.
func CreateErrorResponse(message, errorCode string) response.Response {
	return response.Response{
		Data: map[string]interface{}{
			"error":   true,
			"message": message,
			"code":    errorCode,
		},
		Headers: map[string]string{
			"X-Auth-Error":   errorCode,
			"X-Error-Source": "JWT-Service", 
			"WWW-Authenticate": "Bearer",   
		},
		Metadata: map[string]interface{}{
			"timestamp":   time.Now(),
			"auth_failed": true,
			"error_type":  "authentication",
		},
	}
}

// GetJWTClaims retrieves JWT claims from the GoFr context (if set by a previous middleware or login process).
// Note: For claims validated by app.EnableOAuth(), ctx.GetAuthInfo().GetClaims() in handlers is preferred.
// This function can be used if you manually put claims into the context for other reasons.
func GetJWTClaims(ctx *gofr.Context) (jwt.MapClaims, bool) {
	if claims, ok := ctx.Context.Value(JWTClaim).(jwt.MapClaims); ok {
		return claims, true
	}
	return nil, false
}

// Helper function to get user ID from context
func GetUserID(ctx *gofr.Context) (string, bool) {
	if userID, ok := ctx.Context.Value(UserIDKey).(string); ok {
		return userID, true
	}
	return "", false
}

// Helper function to get username from context
func GetUsername(ctx *gofr.Context) (string, bool) {
	if username, ok := ctx.Context.Value(UsernameKey).(string); ok {
		return username, true
	}
	return "", false
}

// Helper function to get email from context
func GetEmail(ctx *gofr.Context) (string, bool) {
	if email, ok := ctx.Context.Value(EmailKey).(string); ok {
		return email, true
	}
	return "", false
}

// Helper function to get user roles from context
func GetRoles(ctx *gofr.Context) ([]interface{}, bool) {
	if roles, ok := ctx.Context.Value(RolesKey).([]interface{}); ok {
		return roles, true
	}
	return nil, false
}

// Helper function to check if user has a specific role
func HasRole(ctx *gofr.Context, role string) bool {
	roles, exists := GetRoles(ctx)
	if !exists {
		return false
	}

	for _, r := range roles {
		if roleStr, ok := r.(string); ok && roleStr == role {
			return true
		}
	}
	return false
}

// CreateSuccessResponse creates a structured success response with auth metadata.
// This function is used by LoginHandler for consistent success response formats.
func CreateSuccessResponse(data interface{}, ctx *gofr.Context) response.Response {
	// Attempt to get user info from the context for metadata
	userID, _ := GetUserID(ctx)
	username, _ := GetUsername(ctx)

	return response.Response{
		Data: data,
		Headers: map[string]string{
			"X-Auth-Success": "true",
			"X-User-ID":      userID,
		},
		Metadata: map[string]interface{}{
			"timestamp":     time.Now(),
			"authenticated": true,
			"user_id":       userID,
			"username":      username,
		},
	}
}

// RequireRole creates a middleware that checks for specific roles.
// This can still be used for authorization AFTER app.EnableOAuth() has authenticated the user.
func RequireRole(roles ...string) func(gofr.Handler) gofr.Handler {
	return func(next gofr.Handler) gofr.Handler {
		return func(ctx *gofr.Context) (interface{}, error) {
			// Get claims using GoFr's native method for validated claims
			authInfo := ctx.GetAuthInfo()
			if authInfo == nil || authInfo.GetClaims() == nil {
				// User not authenticated or claims missing. This should ideally be caught by app.EnableOAuth() first.
				return CreateErrorResponse("Authentication required for role check", "AUTH_REQUIRED"), nil
			}

			claims := authInfo.GetClaims()
			userRoles, ok := claims["roles"].([]interface{})
			if !ok {
				// Claims do not contain roles or roles are not in the expected format
				ctx.Logger.Warnf("RequireRole: Roles missing or invalid in claims for user: %v", claims["username"])
				return CreateErrorResponse("Insufficient permissions (roles not found)", "INSUFFICIENT_PERMISSIONS"), nil
			}

			hasRequiredRole := false
			for _, requiredRole := range roles {
				for _, actualRole := range userRoles {
					if roleStr, isString := actualRole.(string); isString && roleStr == requiredRole {
						hasRequiredRole = true
						break
					}
				}
				if hasRequiredRole {
					break
				}
			}

			if !hasRequiredRole {
				return CreateErrorResponse("Insufficient permissions", "INSUFFICIENT_PERMISSIONS"), nil
			}

			return next(ctx)
		}
	}
}