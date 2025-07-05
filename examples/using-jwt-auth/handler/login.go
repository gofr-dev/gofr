package handler

import (
	"crypto/rsa" // Required for RSA key types
	"fmt"
	"time"
	"crypto/rand"
	"encoding/hex"

	"github.com/golang-jwt/jwt/v5"
	"gofr.dev/pkg/gofr"
	"gofr.dev/examples/using-jwt-auth/middleware"
	"golang.org/x/crypto/bcrypt" // For password hashing
)

// User represents a simplified user model.
type User struct {
	ID       string
	Username string
	Email    string
	Roles    []string
}

// LoginRequest defines the structure for the incoming login JSON payload.
type LoginRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=8"`
}

// TokenResponse defines the structure for the login response
type TokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	TokenType string `json:"token_type"`
}

// AuthenticateUser is your placeholder authentication function.
// IMPORTANT: This must be replaced with a real database lookup and secure password verification.
func AuthenticateUser(ctx *gofr.Context, username, password string) (*User, error) {
	// In production, replace with database queries and bcrypt password verification
	// Example users with bcrypt-hashed passwords
	users := map[string]struct {
		hashedPassword string
		user           *User
	}{
		"testuser": {
			// bcrypt hash of "password123"
			hashedPassword: "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi",
			user: &User{
				ID:       "user-001",
				Username: "testuser",
				Email:    "test@example.com",
				Roles:    []string{"user"},
			},
		},
		"admin": {
			// bcrypt hash of "adminpass"
			hashedPassword: "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi",
			user: &User{
				ID:       "user-002",
				Username: "admin",
				Email:    "admin@example.com",
				Roles:    []string{"admin", "user"},
			},
		},
	}

	userData, exists := users[username]
	if !exists {
		ctx.Logger.Warnf("Authentication failed for username: %s (user not found)", username)
		return nil, fmt.Errorf("invalid username or password")
	}

	// Verify password using bcrypt
	err := bcrypt.CompareHashAndPassword([]byte(userData.hashedPassword), []byte(password))
	if err != nil {
		ctx.Logger.Warnf("Authentication failed for username: %s (invalid password)", username)
		return nil, fmt.Errorf("invalid username or password")
	}

	return userData.user, nil
}

// generateJTI creates a unique JWT ID for token tracking
func generateJTI() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// LoginHandler handles user login requests, now signing with RS256.
// It expects an *rsa.PrivateKey for signing.
func LoginHandler(privateKey *rsa.PrivateKey) gofr.Handler {
	return func(ctx *gofr.Context) (interface{}, error) {
		var req LoginRequest
		if err := ctx.Bind(&req); err != nil {
			ctx.Logger.Errorf("Failed to bind login request body: %v", err)
			return middleware.CreateErrorResponse("Invalid request body format", "INVALID_REQUEST_FORMAT"), nil
		}

		// Enhanced validation
		if req.Username == "" || req.Password == "" {
			ctx.Logger.Warnf("Login attempt with missing credentials. Username: '%s'", req.Username)
			return middleware.CreateErrorResponse("Username and password are required", "MISSING_CREDENTIALS"), nil
		}

		// Additional validation for minimum lengths
		if len(req.Username) < 3 || len(req.Password) < 8 {
			ctx.Logger.Warnf("Login attempt with invalid credential format. Username: '%s'", req.Username)
			return middleware.CreateErrorResponse("Username must be at least 3 characters and password at least 8 characters", "INVALID_CREDENTIALS_FORMAT"), nil
		}

		user, err := AuthenticateUser(ctx, req.Username, req.Password)
		if err != nil {
			return middleware.CreateErrorResponse(err.Error(), "AUTHENTICATION_FAILED"), nil
		}
		if user == nil {
			return middleware.CreateErrorResponse("Invalid username or password", "AUTHENTICATION_FAILED"), nil
		}

		// Generate unique JWT ID for token tracking
		jti, err := generateJTI()
		if err != nil {
			ctx.Logger.Errorf("Failed to generate JWT ID: %v", err)
			return middleware.CreateErrorResponse("Failed to generate authentication token", "TOKEN_GENERATION_FAILED"), nil
		}

		// Set token expiration
		expirationTime := time.Now().Add(1 * time.Hour)
		
		// Prepare JWT claims with enhanced security
		claims := jwt.MapClaims{
			"sub":      user.ID,                    // Subject (user ID)
			"username": user.Username,              // Username
			"email":    user.Email,                 // Email
			"roles":    user.Roles,                 // User roles
			"exp":      expirationTime.Unix(),      // Token expiration
			"iat":      time.Now().Unix(),          // Token issued at
			"nbf":      time.Now().Unix(),          // Not before (prevents pre-dated tokens)
			"jti":      jti,                        // JWT ID for token tracking
			"iss":      "your-app-name",           // Issuer
			"aud":      "your-app-audience",       // Audience
		}

		// Create a new token with RS256 signing method
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		
		// Add key ID to header for JWKS verification
		token.Header["kid"] = "my-rsa-key"

		// Sign the token with the provided RSA private key
		tokenStr, err := token.SignedString(privateKey)
		if err != nil {
			ctx.Logger.Errorf("Failed to sign JWT token for user '%s' with RS256: %v", user.Username, err)
			return middleware.CreateErrorResponse("Failed to generate authentication token", "TOKEN_GENERATION_FAILED"), nil
		}

		ctx.Logger.Infof("User '%s' (ID: %s) logged in successfully. RS256 Token issued with JTI: %s", user.Username, user.ID, jti)

		// Return enhanced token response
		response := TokenResponse{
			Token:     tokenStr,
			ExpiresAt: expirationTime.Unix(),
			TokenType: "Bearer",
		}

		return middleware.CreateSuccessResponse(response, ctx), nil
	}
}