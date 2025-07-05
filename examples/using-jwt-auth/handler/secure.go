package handler

import (
	"fmt" 
	"gofr.dev/pkg/gofr"
	"gofr.dev/examples/using-jwt-auth/middleware" 
)

// SecureResponse represents the response from a protected endpoint
type SecureResponse struct {
	Message  string `json:"message"`
	Username string `json:"username"`
	Access   string `json:"access"`
}

// SecureHandler handles requests to protected routes
func SecureHandler(ctx *gofr.Context) (interface{}, error) {
	// Attempt to retrieve claims using GoFr's structured method
	// This will access claims that were set by GoFr's OAuth middleware (or your custom JWTAuth middleware)
	// into the standard context using the middleware.JWTClaim key.
	authInfo := ctx.GetAuthInfo()

	// It's good practice to check if authInfo is nil, though it should ideally be populated by the middleware.
	if authInfo == nil {
		ctx.Logger.Warn("SecureHandler: AuthInfo is nil. User likely not authenticated via GoFr's standard auth mechanism.")
		return middleware.CreateErrorResponse("Authentication information not found", "AUTH_INFO_MISSING"), nil
	}

	// GetClaims() returns jwt.MapClaims (which is map[string]interface{})
	claims := authInfo.GetClaims()

	// Log the retrieved claims for debugging purposes
	ctx.Logger.Debugf("SecureHandler: Retrieved claims: %+v", claims)

	// Ensure claims are not nil. While GetClaims should return MapClaims, it could be nil if not set properly.
	if claims == nil {
		ctx.Logger.Warn("SecureHandler: Claims from AuthInfo are nil. Check JWTAuth middleware integration with GoFr.")
		return middleware.CreateErrorResponse("Invalid or missing claims in authentication info", "CLAIMS_MISSING_OR_INVALID"), nil
	}

	// Access the username directly from the 'claims' map.
	// We expect 'username' to be a string, as set during token generation in LoginHandler.
	username, ok := claims["username"].(string)
	if !ok || username == "" {
		ctx.Logger.Warn("SecureHandler: Username missing or invalid in claims.")
		return middleware.CreateErrorResponse("Invalid or missing username in JWT claims", "USERNAME_MISSING"), nil
	}

	// Optionally, retrieve other details like roles from claims for more fine-grained access if needed
	// Example:
	// roles, ok := claims["roles"].([]interface{})
	// if ok {
	//     ctx.Logger.Debugf("User roles: %+v", roles)
	// }

	// Return a structured success response with user information
	return SecureResponse{
		Message:  "Access granted to protected resource",
		Username: username,
		Access:   fmt.Sprintf("This is a protected endpoint. Authenticated as: %s", username),
	}, nil
}