package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"golang.org/x/oauth2"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
)

func TestAuthProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	validBasicAuthConfig := &BasicAuthConfig{UserName: "username", Password: "password"}
	basicAuthServer := setupBasicAuthHTTPServer(t, validBasicAuthConfig)
	invalidBasicAuthConfig := &BasicAuthConfig{UserName: "username", Password: "wrong-password"}

	validOAuthConfig := oAuthConfigForTests(t, "")
	oAuthServer := setupOAuthHTTPServer(t, validOAuthConfig)
	validOAuthConfig.TokenURL = oAuthServer.URL + "/token"

	invalidOAuthConfig := oAuthConfigForTests(t, oAuthServer.URL+"/token")
	invalidOAuthConfig2 := oAuthConfigForTests(t, "")
	invalidOAuthConfig3 := oAuthConfigForTests(t, invalidURL)

	validAPIKeyConfig := &APIKeyConfig{"valid-value"}
	apiKeyAuthServer := setupAPIKeyAuthHTTPServer(t, validAPIKeyConfig)
	invalidAPIKeyConfig := &APIKeyConfig{"invalid-value"}

	authHeaderExistsErr := service.AuthErr{Message: "value auth-string already exists for header " + AuthHeader}
	apiHeaderExistsErr := service.AuthErr{Message: "value auth-string already exists for header " + xAPIKeyHeader}

	testCases := []struct {
		authOption service.Options
		headers    map[string]string
		statusCode int
		err        error
	}{
		{authOption: validBasicAuthConfig, statusCode: http.StatusOK},
		{authOption: invalidBasicAuthConfig, statusCode: http.StatusUnauthorized},
		{authOption: validOAuthConfig, headers: map[string]string{AuthHeader: "auth-string"}, err: authHeaderExistsErr},

		{authOption: validAPIKeyConfig, statusCode: http.StatusOK},
		{authOption: invalidAPIKeyConfig, statusCode: http.StatusUnauthorized},
		{authOption: validAPIKeyConfig, headers: map[string]string{xAPIKeyHeader: "auth-string"}, err: apiHeaderExistsErr},

		{authOption: validOAuthConfig, statusCode: http.StatusOK},
		{authOption: invalidOAuthConfig, statusCode: http.StatusUnauthorized, err: errInvalidCredentials},
		{authOption: invalidOAuthConfig2, statusCode: http.StatusUnauthorized, err: errMissingTokenURL},
		{authOption: invalidOAuthConfig3, statusCode: http.StatusUnauthorized, err: errIncorrectProtocol},
		{authOption: validOAuthConfig, headers: map[string]string{AuthHeader: "auth-string"}, err: authHeaderExistsErr},
	}

	httpMethods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test Case #%d", i), func(t *testing.T) {
			var server *httptest.Server
			switch tc.authOption.(type) {
			case *OAuthConfig:
				server = oAuthServer
			case *BasicAuthConfig:
				server = basicAuthServer
			case *APIKeyConfig:
				server = apiKeyAuthServer
			}

			httpService := service.NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil, tc.authOption)

			for _, method := range httpMethods {
				resp, err := callHTTPService(t.Context(), httpService, method, tc.headers)

				validateOAuthError(t, err, tc.err, tc.statusCode)

				if err != nil {
					return
				}

				assert.Equal(t, tc.statusCode, resp.StatusCode)

				err = resp.Body.Close()
				if err != nil {
					t.Errorf("error closing response body %v", err)
				}
			}
		})
	}
}

func validateOAuthError(t *testing.T, err, expectedError error, statusCode int) {
	t.Helper()

	retrieveError := &oauth2.RetrieveError{}
	URLError := &url.Error{}
	authErr := service.AuthErr{}

	if errors.As(err, &retrieveError) {
		assert.Equal(t, statusCode, retrieveError.Response.StatusCode)
	} else if errors.As(err, &URLError) {
		assert.Equal(t, expectedError, URLError.Err)
	} else if errors.As(err, &authErr) {
		assert.Equal(t, expectedError, err)
	} else if err != nil {
		t.Errorf("Unknown error type encountered %v", err)
	}
}

func callHTTPService(ctx context.Context, service service.HTTP, method string,
	headers map[string]string) (resp *http.Response, err error) {
	if headers != nil {
		resp, err = callHTTPServiceWithHeaders(ctx, service, method, headers)
	} else {
		resp, err = callHTTPServiceWithoutHeaders(ctx, service, method)
	}

	return resp, err
}

func callHTTPServiceWithHeaders(ctx context.Context, s service.HTTP, method string,
	headers map[string]string) (resp *http.Response, err error) {
	path := "test"
	queryParams := map[string]any{"key": "value"}
	body := []byte("body")

	switch method {
	case http.MethodGet:
		return s.GetWithHeaders(ctx, path, queryParams, headers)
	case http.MethodPost:
		return s.PostWithHeaders(ctx, path, queryParams, body, headers)
	case http.MethodPut:
		return s.PutWithHeaders(ctx, path, queryParams, body, headers)
	case http.MethodPatch:
		return s.PatchWithHeaders(ctx, path, queryParams, body, headers)
	case http.MethodDelete:
		return s.DeleteWithHeaders(ctx, path, body, headers)
	default:
		return nil, service.AuthErr{Message: "unknown method"}
	}
}

func callHTTPServiceWithoutHeaders(ctx context.Context, s service.HTTP, method string) (resp *http.Response, err error) {
	path := "test"
	queryParams := map[string]any{"key": "value"}
	body := []byte("body")

	switch method {
	case http.MethodGet:
		return s.Get(ctx, path, queryParams)
	case http.MethodPost:
		return s.Post(ctx, path, queryParams, body)
	case http.MethodPut:
		return s.Put(ctx, path, queryParams, body)
	case http.MethodPatch:
		return s.Patch(ctx, path, queryParams, body)
	case http.MethodDelete:
		return s.Delete(ctx, path, body)
	default:
		return nil, service.AuthErr{Message: "unknown method"}
	}
}
