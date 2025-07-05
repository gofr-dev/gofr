package handler

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"gofr.dev/examples/using-jwt-auth/middleware"
	"gofr.dev/pkg/gofr"
)

// User represents a simplified user model.
type User struct {
	ID       string
	Username string
	Email    string
	Roles    []string
}

type LoginRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=8"`
}

type TokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	TokenType string `json:"token_type"`
}

func AuthenticateUser(ctx *gofr.Context, username, password string) (*User, error) {
	users := map[string]struct {
		hashedPassword string
		user           *User
	}{
		"testuser": {
			hashedPassword: "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi",
			user: &User{
				ID:       "user-001",
				Username: "testuser",
				Email:    "test@example.com",
				Roles:    []string{"user"},
			},
		},
		"admin": {
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
	if !exists || bcrypt.CompareHashAndPassword([]byte(userData.hashedPassword), []byte(password)) != nil {
		ctx.Logger.Warnf("Authentication failed for username: %s", username)
		return nil, fmt.Errorf("invalid username or password")
	}

	return userData.user, nil
}

func generateJTI() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func validateLoginRequest(ctx *gofr.Context, req *LoginRequest) error {
	if err := ctx.Bind(req); err != nil {
		ctx.Logger.Errorf("Failed to bind login request body: %v", err)
		return fmt.Errorf("INVALID_REQUEST_FORMAT")
	}

	if req.Username == "" || req.Password == "" {
		ctx.Logger.Warnf("Missing credentials. Username: '%s'", req.Username)
		return fmt.Errorf("MISSING_CREDENTIALS")
	}

	if len(req.Username) < 3 || len(req.Password) < 8 {
		ctx.Logger.Warnf("Invalid credential format. Username: '%s'", req.Username)
		return fmt.Errorf("INVALID_CREDENTIALS_FORMAT")
	}

	return nil
}

func createJWT(user *User, privateKey *rsa.PrivateKey, jti string, expiration time.Time) (string, error) {
	claims := jwt.MapClaims{
		"sub":      user.ID,
		"username": user.Username,
		"email":    user.Email,
		"roles":    user.Roles,
		"exp":      expiration.Unix(),
		"iat":      time.Now().Unix(),
		"nbf":      time.Now().Unix(),
		"jti":      jti,
		"iss":      "your-app-name",
		"aud":      "your-app-audience",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "my-rsa-key"

	return token.SignedString(privateKey)
}

func LoginHandler(privateKey *rsa.PrivateKey) gofr.Handler {
	return func(ctx *gofr.Context) (interface{}, error) {
		var req LoginRequest
		if err := validateLoginRequest(ctx, &req); err != nil {
			return middleware.CreateErrorResponse(err.Error(), err.Error()), nil
		}

		user, err := AuthenticateUser(ctx, req.Username, req.Password)
		if err != nil {
			return middleware.CreateErrorResponse(err.Error(), "AUTHENTICATION_FAILED"), nil
		}

		jti, err := generateJTI()
		if err != nil {
			ctx.Logger.Errorf("Failed to generate JWT ID: %v", err)
			return middleware.CreateErrorResponse("Failed to generate authentication token", "TOKEN_GENERATION_FAILED"), nil
		}

		expirationTime := time.Now().Add(1 * time.Hour)
		tokenStr, err := createJWT(user, privateKey, jti, expirationTime)
		if err != nil {
			ctx.Logger.Errorf("JWT signing failed for user '%s': %v", user.Username, err)
			return middleware.CreateErrorResponse("Failed to generate authentication token", "TOKEN_GENERATION_FAILED"), nil
		}

		ctx.Logger.Infof("User '%s' (ID: %s) logged in successfully. Token JTI: %s", user.Username, user.ID, jti)

		response := TokenResponse{
			Token:     tokenStr,
			ExpiresAt: expirationTime.Unix(),
			TokenType: "Bearer",
		}

		return middleware.CreateSuccessResponse(response, ctx), nil
	}
}
