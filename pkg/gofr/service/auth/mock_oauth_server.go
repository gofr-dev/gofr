package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const clientIDLength = 10
const clientSecretLength = 24
const privateKeyBits = 2048

type oAuthTestSever struct {
	tokenURL      string
	clientID      string
	clientSecret  string
	testURL       string
	audienceClaim string
	privateKey    *rsa.PrivateKey
	httpServer    *httptest.Server
}

func setupOAuthHTTPServer(t *testing.T, config *OAuthConfig) *httptest.Server {
	t.Helper()

	server := oAuthTestSever{
		tokenURL:      "/token",
		testURL:       "/test",
		audienceClaim: config.EndpointParams.Get("aud"),
	}

	server.clientID = config.ClientID
	server.clientSecret = config.ClientSecret

	privateKey, err := rsa.GenerateKey(rand.Reader, privateKeyBits)
	require.NoError(t, err, "failed to generate private key, aborting")

	server.privateKey = privateKey

	mux := http.NewServeMux()

	mux.HandleFunc(server.tokenURL, func(w http.ResponseWriter, r *http.Request) {
		errMessage, statusCode := server.validateCredentials(r)

		if statusCode != http.StatusOK {
			http.Error(w, errMessage, statusCode)
			return
		}

		accessToken, err := server.generateToken(getClaims(r))

		if err != nil {
			http.Error(w, "Unable to generate token", http.StatusInternalServerError)
			return
		}

		// Prepare the JSON response
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")

		tokenResponse := map[string]any{
			"access_token": accessToken,
			"token_type":   "Bearer",
			"expires_in":   3600,         // Expires in 1 hour
			"scope":        "read write", // Mock scope
		}

		_ = json.NewEncoder(w).Encode(tokenResponse)
	})

	mux.HandleFunc(server.testURL, func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get(AuthHeader)
		token := strings.Split(header, " ")

		if len(token) <= 1 {
			w.WriteHeader(http.StatusUnauthorized)
		}

		parsedToken, _ := jwt.Parse(token[1], func(*jwt.Token) (any, error) {
			return []byte("my-secret-key"), nil
		})

		claims, err := parsedToken.Claims.GetAudience()
		assert.NoError(t, err, "error while getting audience from claims")
		assert.NotEmptyf(t, claims, "no value in claims")

		assert.Equal(t, server.audienceClaim, claims[0])

		w.WriteHeader(http.StatusOK)
	})

	server.httpServer = httptest.NewServer(mux)

	t.Cleanup(func() {
		server.httpServer.Close()
	})

	return server.httpServer
}

func (s *oAuthTestSever) validateCredentials(r *http.Request) (errMessage string, statusCode int) {
	err := r.ParseForm()
	if err != nil {
		return "Failed to parse form", http.StatusBadRequest
	}

	grantType := r.Form.Get("grant_type")
	clientID := r.Form.Get("client_id")
	clientSecret := r.Form.Get("client_secret")

	// Basic validation
	if grantType != "client_credentials" || clientID == "" || clientSecret == "" {
		return "Invalid token request", http.StatusBadRequest
	}

	// Validate the authorization code
	if s.clientID != clientID || s.clientSecret != clientSecret {
		return "Invalid credentials", http.StatusUnauthorized
	}

	return "", http.StatusOK
}

func (s *oAuthTestSever) generateToken(claims jwt.MapClaims) (string, error) {
	claims["iat"] = time.Now().Unix()
	claims["exp"] = time.Now().Add(time.Hour).Unix()
	t := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)

	return t.SignedString(s.privateKey)
}

func getClaims(r *http.Request) map[string]any {
	claims := make(map[string]any, 0)

	for key, value := range r.Form {
		if key == "client_id" || key == "client_secret" || key == "grant_type" {
			continue
		}

		claims[key] = value
	}

	return claims
}

// Helper function to generate a random string.
func generateRandomString(length int) (token string, err error) {
	// Generate random bytes
	b := make([]byte, length)

	_, err = rand.Read(b) // Use crypto/rand.Read
	if err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}

	// Encode to base64 to make it URL-safe and human-readable (for tokens)
	return base64.URLEncoding.EncodeToString(b), nil
}
