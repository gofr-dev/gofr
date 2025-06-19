package service

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

var err1 = errors.New(`unsupported protocol scheme ""`)

var err2 = errors.New(`unsupported protocol scheme "abc"`)

func TestHttpService_RequestsOAuth(t *testing.T) {
	server := setupOAuthHTTPServer(t)
	defer server.httpServer.Close()

	tokenURL := server.getTokenURL()

	invalidURL := "abc://invalid-url"

	testCases := []struct {
		method     string
		headers    bool
		tokenURL   string
		err        error
		statusCode int
	}{
		{http.MethodGet, false, tokenURL, nil, http.StatusOK},
		{http.MethodPost, false, tokenURL, nil, http.StatusOK},
		{http.MethodPut, false, tokenURL, nil, http.StatusOK},
		{http.MethodDelete, false, tokenURL, nil, http.StatusOK},
		{http.MethodPatch, false, tokenURL, nil, http.StatusOK},
		{http.MethodGet, false, "", &url.Error{Op: "Post", URL: "", Err: err1}, http.StatusOK},
		{http.MethodPost, false, "", &url.Error{Op: "Post", URL: "", Err: err1}, http.StatusOK},
		{http.MethodPut, false, "", &url.Error{Op: "Post", URL: "", Err: err1}, http.StatusOK},
		{http.MethodDelete, false, "", &url.Error{Op: "Post", URL: "", Err: err1}, http.StatusOK},
		{http.MethodPatch, false, "", &url.Error{Op: "Post", URL: "", Err: err1}, http.StatusOK},
		{http.MethodGet, false, invalidURL, &url.Error{Op: "Post", URL: invalidURL, Err: err2}, http.StatusOK},
		{http.MethodPost, false, invalidURL, &url.Error{Op: "Post", URL: invalidURL, Err: err2}, http.StatusOK},
		{http.MethodPut, false, invalidURL, &url.Error{Op: "Post", URL: invalidURL, Err: err2}, http.StatusOK},
		{http.MethodDelete, false, invalidURL, &url.Error{Op: "Post", URL: invalidURL, Err: err2}, http.StatusOK},
		{http.MethodPatch, false, invalidURL, &url.Error{Op: "Post", URL: invalidURL, Err: err2}, http.StatusOK},
		{http.MethodGet, true, tokenURL, nil, http.StatusOK},
		{http.MethodPost, true, tokenURL, nil, http.StatusOK},
		{http.MethodPut, true, tokenURL, nil, http.StatusOK},
		{http.MethodDelete, true, tokenURL, nil, http.StatusOK},
		{http.MethodPatch, true, tokenURL, nil, http.StatusOK},
		{http.MethodGet, true, "", &url.Error{Op: "Post", URL: "", Err: err1}, http.StatusOK},
		{http.MethodPost, true, "", &url.Error{Op: "Post", URL: "", Err: err1}, http.StatusOK},
		{http.MethodPut, true, "", &url.Error{Op: "Post", URL: "", Err: err1}, http.StatusOK},
		{http.MethodDelete, true, "", &url.Error{Op: "Post", URL: "", Err: err1}, http.StatusOK},
		{http.MethodPatch, true, "", &url.Error{Op: "Post", URL: "", Err: err1}, http.StatusOK},
		{http.MethodGet, true, invalidURL, &url.Error{Op: "Post", URL: invalidURL, Err: err2}, http.StatusOK},
		{http.MethodPost, true, invalidURL, &url.Error{Op: "Post", URL: invalidURL, Err: err2}, http.StatusOK},
		{http.MethodPut, true, invalidURL, &url.Error{Op: "Post", URL: invalidURL, Err: err2}, http.StatusOK},
		{http.MethodDelete, true, invalidURL, &url.Error{Op: "Post", URL: invalidURL, Err: err2}, http.StatusOK},
		{http.MethodPatch, true, invalidURL, &url.Error{Op: "Post", URL: invalidURL, Err: err2}, http.StatusOK},
	}

	for i, tc := range testCases {
		var resp *http.Response

		var err error

		service := server.httpService()
		service = getOAuthService(service, server.clientID, server.clientSecret, tc.tokenURL, server.audienceClaim)

		switch tc.method {
		case http.MethodGet:
			resp, err = callHTTPServiceGet(t.Context(), service, tc.headers)
		case http.MethodPost:
			resp, err = callHTTPServicePost(t.Context(), service, tc.headers)

		case http.MethodDelete:
			resp, err = callHTTPServiceDelete(t.Context(), service, tc.headers)

		case http.MethodPut:
			resp, err = callHTTPServicePut(t.Context(), service, tc.headers)

		case http.MethodPatch:
			resp, err = callHTTPServicePatch(t.Context(), service, tc.headers)
		}

		assert.Equalf(t, tc.err, err, "failed test case #%d", i)

		if resp != nil {
			assert.Equalf(t, tc.statusCode, resp.StatusCode, "failed test case #%d", i)

			if err = resp.Body.Close(); err != nil {
				t.Logf("error in closing response %v", err)
			}
		}
	}
}

func callHTTPServiceGet(ctx context.Context, service HTTP, headers bool) (resp *http.Response, err error) {
	if headers {
		resp, err = service.GetWithHeaders(ctx, "test", nil, nil)
	} else {
		resp, err = service.Get(ctx, "test", nil)
	}

	return resp, err
}

func callHTTPServicePost(ctx context.Context, service HTTP, headers bool) (resp *http.Response, err error) {
	if headers {
		resp, err = service.PostWithHeaders(ctx, "test", nil, nil, nil)
	} else {
		resp, err = service.Post(ctx, "test", nil, nil)
	}

	return resp, err
}

func callHTTPServicePut(ctx context.Context, service HTTP, headers bool) (resp *http.Response, err error) {
	if headers {
		resp, err = service.PutWithHeaders(ctx, "test", nil, nil, nil)
	} else {
		resp, err = service.Put(ctx, "test", nil, nil)
	}

	return resp, err
}

func callHTTPServicePatch(ctx context.Context, service HTTP, headers bool) (resp *http.Response, err error) {
	if headers {
		resp, err = service.PatchWithHeaders(ctx, "test", nil, nil, nil)
	} else {
		resp, err = service.Patch(ctx, "test", nil, nil)
	}

	return resp, err
}

func callHTTPServiceDelete(ctx context.Context, service HTTP, headers bool) (resp *http.Response, err error) {
	if headers {
		resp, err = service.DeleteWithHeaders(ctx, "test", nil, nil)
	} else {
		resp, err = service.Delete(ctx, "test", nil)
	}

	return resp, err
}

func TestHttpService_NewOAuthConfig(t *testing.T) {
	server := setupOAuthHTTPServer(t)
	defer server.httpServer.Close()

	tokenURL := server.getTokenURL()
	clientID := server.clientID
	clientSecret := server.clientSecret

	testCases := []struct {
		clientID     string
		clientSecret string
		tokenURL     string
		scopes       []string
		params       url.Values
		authStyle    oauth2.AuthStyle
		err          error
	}{
		{"", "", "", nil, nil, 0, OAuthErr{nil, "client id is mandatory"}},
		{clientID, "", "", nil, nil, 0, OAuthErr{nil, "client secret is mandatory"}},
		{clientID, "", tokenURL, nil, nil, 0, OAuthErr{nil, "client secret is mandatory"}},
		{clientID, clientSecret, "", nil, nil, 0, OAuthErr{nil, "token url is mandatory"}},
		{clientID, clientSecret, "invalid_url_format", nil, nil, 0, OAuthErr{nil, "empty host"}},
		{clientID, clientSecret, tokenURL, nil, nil, 0, nil},
		{clientID, "some_random_client_secret", tokenURL, nil, nil, 0, nil},
		{"some_random_client_id", clientSecret, tokenURL, nil, nil, 0, nil},
		{clientID, clientSecret, tokenURL, nil, nil, 1, nil},
		{clientID, "some_random_client_secret", tokenURL, nil, nil, 1, nil},
		{"some_random_client_id", clientSecret, tokenURL, nil, nil, 2, nil},
	}

	for i, tc := range testCases {
		config, err := NewOAuthConfig(tc.clientID, tc.clientSecret, tc.tokenURL, tc.scopes, tc.params, tc.authStyle)
		assert.Equal(t, tc.err, err, "failed test case #%d", i)

		if tc.err != nil {
			assert.Empty(t, config, "failed test case #%d", i)
			continue
		}

		oAuthConfig, ok := config.(*OAuthConfig)
		assert.True(t, ok, "failed to get OAuthConfig for testcase #%d", i)
		assert.Equal(t, tc.clientID, oAuthConfig.ClientID, "failed test case #%d", i)
		assert.Equal(t, tc.clientSecret, oAuthConfig.ClientSecret, "failed test case #%d", i)
		assert.Equal(t, tc.tokenURL, oAuthConfig.TokenURL, "failed test case #%d", i)
		assert.Equal(t, tc.params, oAuthConfig.EndpointParams, "failed test case #%d", i)
		assert.Equal(t, tc.scopes, oAuthConfig.Scopes, "failed test case #%d", i)
		assert.Equal(t, tc.authStyle, oAuthConfig.AuthStyle, "failed test case #%d", i)
	}
}

func TestHttpService_validateTokenURL(t *testing.T) {
	testCases := []struct {
		tokenURL string
		errMsg   string
	}{
		{"https://www.example.com", ""},
		{"https://www.example.com.", "invalid host pattern, ends with `.`"},
		{"https://www.192.168.1.1.com", ""},
		{"https://www.192.168.1.1..com", "invalid host pattern, contains `..`"},
		{"ftp://www.192.168.1.1..com", "invalid host pattern, contains `..`"},
		{"ftp://www.192.168.1.1.com", "invalid scheme, allowed http and https only"},
		{"www.192.168.1.1.com", "empty host"},
		{"https://www.example.", "invalid host pattern, ends with `.`"},
		{"", "token url is mandatory"},
		{"invalid_url_format", "empty host"},
	}

	for i, tc := range testCases {
		err := validateTokenURL(tc.tokenURL)
		if tc.errMsg != "" {
			assert.ErrorContainsf(t, err, tc.errMsg, "failed test case #%d", i)
		}
	}
}

func TestHttpService_addAuthorizationHeader(t *testing.T) {
	server := setupOAuthHTTPServer(t)
	defer server.httpServer.Close()

	tokenURL := server.getTokenURL()

	emptyHeaders := map[string]string{}
	headerWithAuth := map[string]string{AuthHeader: "Value"}
	headerWithEmptyAuth := map[string]string{AuthHeader: ""}
	headerWithoutAuth := map[string]string{"Content Type": "Value"}
	headerWithEmptyAuthAndOtherValues := map[string]string{"Content Type": "Value", AuthHeader: ""}
	tokenURLError := &url.Error{Op: "Post", URL: "", Err: err1}
	authHeaderExistsError := OAuthErr{Message: "auth header already exists Value"}

	testCases := []struct {
		ctx      context.Context
		tokenURL string
		headers  map[string]string
		response map[string]string
		err      error
	}{
		{t.Context(), "", headerWithAuth, nil, authHeaderExistsError},
		{t.Context(), "", nil, nil, tokenURLError},
		{t.Context(), tokenURL, headerWithAuth, nil, authHeaderExistsError},
		{t.Context(), tokenURL, headerWithEmptyAuth, emptyHeaders, nil},
		{t.Context(), tokenURL, headerWithoutAuth, headerWithoutAuth, nil},
		{t.Context(), tokenURL, headerWithEmptyAuthAndOtherValues, headerWithoutAuth, nil},
	}

	for i, tc := range testCases {
		service, ok := getOAuthService(server.httpService(), server.clientID, server.clientSecret, tc.tokenURL, server.audienceClaim).(*oAuth)
		assert.True(t, ok, "unable to get oAuth object for test case #%d", i)

		headers, err := service.addAuthorizationHeader(tc.ctx, tc.headers)
		assert.Equal(t, tc.err, err, "failed test case #%d", i)

		if err == nil {
			authHeader, ok := headers[AuthHeader]
			assert.True(t, ok, "failed test case #%d", i)
			assert.NotEmptyf(t, authHeader, "failed test case #%d", i)
			assert.True(t, strings.HasPrefix(authHeader, "Bearer"))
			delete(headers, AuthHeader)
			assert.Equal(t, tc.response, headers, "failed test case #%d", i)
		}
	}
}
