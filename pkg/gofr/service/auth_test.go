package service

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/logging"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	validBasicAuth, invalidBasicAuth := "username:password", "username:wrong-password"
	basicAuthConfig := &BasicAuthConfig{UserName: "username", Password: "password"}
	oAuthConfig := &OAuthConfig{ClientID: "client-id", ClientSecret: "clientSecret"}

	validAPIKey, invalidAPIKey := "valid-value", "invalid-value"
	apiKeyConfig := &APIKeyConfig{validAPIKey}

	testCases := []struct {
		headers    bool
		authOption Options
		httpMethod string
		authHeader string
		statusCode int
	}{
		{authOption: basicAuthConfig, httpMethod: http.MethodPost, authHeader: validBasicAuth, statusCode: http.StatusCreated},
		{authOption: basicAuthConfig, httpMethod: http.MethodGet, authHeader: validBasicAuth, statusCode: http.StatusOK},
		{authOption: basicAuthConfig, httpMethod: http.MethodDelete, authHeader: validBasicAuth, statusCode: http.StatusNoContent},
		{authOption: basicAuthConfig, httpMethod: http.MethodPatch, authHeader: validBasicAuth, statusCode: http.StatusOK},
		{authOption: basicAuthConfig, httpMethod: http.MethodPut, authHeader: validBasicAuth, statusCode: http.StatusOK},
		{authOption: basicAuthConfig, httpMethod: http.MethodPost, authHeader: invalidBasicAuth, statusCode: http.StatusUnauthorized},
		{authOption: basicAuthConfig, httpMethod: http.MethodGet, authHeader: invalidBasicAuth, statusCode: http.StatusUnauthorized},
		{authOption: basicAuthConfig, httpMethod: http.MethodDelete, authHeader: invalidBasicAuth, statusCode: http.StatusUnauthorized},
		{authOption: basicAuthConfig, httpMethod: http.MethodPatch, authHeader: invalidBasicAuth, statusCode: http.StatusUnauthorized},
		{authOption: basicAuthConfig, httpMethod: http.MethodPut, authHeader: invalidBasicAuth, statusCode: http.StatusUnauthorized},

		{authOption: oAuthConfig, httpMethod: http.MethodPost, authHeader: "", statusCode: http.StatusCreated},
		{authOption: oAuthConfig, httpMethod: http.MethodGet, authHeader: "", statusCode: http.StatusOK},
		{authOption: oAuthConfig, httpMethod: http.MethodDelete, authHeader: "", statusCode: http.StatusNoContent},
		{authOption: oAuthConfig, httpMethod: http.MethodPatch, authHeader: "", statusCode: http.StatusOK},
		{authOption: oAuthConfig, httpMethod: http.MethodPut, authHeader: "", statusCode: http.StatusOK},
		{authOption: oAuthConfig, httpMethod: http.MethodPost, authHeader: "", statusCode: http.StatusUnauthorized},
		{authOption: oAuthConfig, httpMethod: http.MethodGet, authHeader: "", statusCode: http.StatusUnauthorized},
		{authOption: oAuthConfig, httpMethod: http.MethodDelete, authHeader: "", statusCode: http.StatusUnauthorized},
		{authOption: oAuthConfig, httpMethod: http.MethodPatch, authHeader: "", statusCode: http.StatusUnauthorized},
		{authOption: oAuthConfig, httpMethod: http.MethodPut, authHeader: "", statusCode: http.StatusUnauthorized},

		{authOption: apiKeyConfig, httpMethod: http.MethodPost, authHeader: validAPIKey, statusCode: http.StatusCreated},
		{authOption: apiKeyConfig, httpMethod: http.MethodGet, authHeader: validAPIKey, statusCode: http.StatusOK},
		{authOption: apiKeyConfig, httpMethod: http.MethodDelete, authHeader: validAPIKey, statusCode: http.StatusNoContent},
		{authOption: apiKeyConfig, httpMethod: http.MethodPatch, authHeader: validAPIKey, statusCode: http.StatusOK},
		{authOption: apiKeyConfig, httpMethod: http.MethodPut, authHeader: validAPIKey, statusCode: http.StatusOK},
		{authOption: apiKeyConfig, httpMethod: http.MethodPost, authHeader: invalidAPIKey, statusCode: http.StatusUnauthorized},
		{authOption: apiKeyConfig, httpMethod: http.MethodGet, authHeader: invalidAPIKey, statusCode: http.StatusUnauthorized},
		{authOption: apiKeyConfig, httpMethod: http.MethodDelete, authHeader: invalidAPIKey, statusCode: http.StatusUnauthorized},
		{authOption: apiKeyConfig, httpMethod: http.MethodPatch, authHeader: invalidAPIKey, statusCode: http.StatusUnauthorized},
		{authOption: apiKeyConfig, httpMethod: http.MethodPut, authHeader: invalidAPIKey, statusCode: http.StatusUnauthorized},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test Case #%d", i), func(t *testing.T) {

			server := setupHTTPServer(t, tc.authOption)

			httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil, tc.authOption)

			var (
				resp *http.Response
				err  error
			)
			if tc.headers {
				resp, err = callHTTPServiceWithHeaders(t.Context(), httpService, tc.httpMethod)
			} else {
				resp, err = callHTTPServiceWithoutHeaders(t.Context(), httpService, tc.httpMethod)
			}

			require.NoError(t, err)
			assert.Equal(t, tc.statusCode, resp.StatusCode)

			err = resp.Body.Close()
			if err != nil {
				t.Errorf("error closing response body %v", err)
			}
		})
	}
}

func setupHTTPServer(t *testing.T, authOption Options) *httptest.Server {
	switch authOption.(type) {
	case *BasicAuthConfig:
		return nil
	case *OAuthConfig:
		return nil
	case *APIKeyConfig:
		return nil
	default:
		return nil
	}
}

func checkAuthHeaders(t *testing.T, r *http.Request) {

}
