package middleware

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/container"
)

func TestNewBasicAuthProvider(t *testing.T) {
	testCases := []struct {
		users    map[string]string
		provider AuthProvider
		err      error
	}{
		{err: errUserListEmpty},
		{users: map[string]string{"user": "password"}, provider: &BasicAuthProvider{Users: map[string]string{"user": "password"}}},
	}
	for _, tc := range testCases {
		authProvider, err := NewBasicAuthProvider(tc.users)
		assert.Equal(t, tc.provider, authProvider)
		assert.Equal(t, tc.err, err)
	}
}

func TestNewBasicAuthProviderWithValidateFunc(t *testing.T) {
	validateFunc := func(_ *container.Container, _, _ string) bool { return true }

	testCases := []struct {
		c            *container.Container
		validateFunc func(c *container.Container, username, password string) bool

		err error
	}{
		{err: errValidateFuncEmpty},
		{validateFunc: validateFunc, err: errContainerNil},
		{c: &container.Container{}, validateFunc: validateFunc},
	}
	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			authProvider, err := NewBasicAuthProviderWithValidateFunc(tc.c, tc.validateFunc)
			assert.Equal(t, tc.err, err)

			if err != nil {
				return
			}

			assert.NotNil(t, authProvider)
		})
	}
}

func TestBasicAuthMiddleware_extractAuthHeader(t *testing.T) {
	users := map[string]string{"storedUser": "storedPass", "user": "password", "newUser": ""}

	testCases := []struct {
		header   string
		response any
		err      error
	}{
		{
			err: ErrorMissingAuthHeader{key: headerAuthorization},
		},
		{
			header: "Basic wrong-header",
			err: ErrorInvalidAuthorizationHeaderFormat{
				key:        headerAuthorization,
				errMessage: "credentials should be in the format base64(username:password)",
			},
			response: "",
		},
		{
			header:   "Basic " + base64.StdEncoding.EncodeToString([]byte("storedUser:storedPass")),
			response: "storedUser",
		},
	}
	provider := &BasicAuthProvider{Users: users}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Header.Set(headerAuthorization, tc.header)
			response, err := provider.ExtractAuthHeader(req)
			assert.Equal(t, tc.response, response)
			assert.Equal(t, tc.err, err)
		})
	}
}

func TestBasicAuthProvider_getAuthMethod(t *testing.T) {
	provider := BasicAuthProvider{}
	assert.Equal(t, Username, provider.GetAuthMethod())
}

func TestParseBasicAuth(t *testing.T) {
	validHeader := base64.StdEncoding.EncodeToString([]byte("user:password"))
	invalidHeader := base64.StdEncoding.EncodeToString([]byte("userpassword"))
	testCases := []struct {
		name         string
		authHeader   string
		expectedUser string
		expectedPass string
		expectedOk   bool
	}{
		{name: "Valid Basic Auth", authHeader: validHeader, expectedUser: "user", expectedPass: "password", expectedOk: true},
		{name: "Invalid Encoding", authHeader: "invalid_base64", expectedUser: "", expectedPass: "", expectedOk: false},
		{name: "Missing Colon Separator", authHeader: invalidHeader, expectedUser: "", expectedPass: "", expectedOk: false},
		{name: "Empty Authorization Header", authHeader: "", expectedUser: "", expectedPass: "", expectedOk: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			username, password, ok := parseBasicAuth(tc.authHeader)

			assert.Equal(t, tc.expectedOk, ok)
			assert.Equal(t, tc.expectedUser, username)
			assert.Equal(t, tc.expectedPass, password)
		})
	}
}

func TestBasicAuthMiddleware_validateCredentials(t *testing.T) {
	users := map[string]string{"storedUser": "storedPass", "user": "password", "newUser": ""}
	validateFuncPass := func(_, _ string) bool { return true }
	validateFuncFail := func(_, _ string) bool { return false }

	validateFuncDataStorePass := func(_ *container.Container, _, _ string) bool { return true }
	validateFuncDataStoreFail := func(_ *container.Container, _, _ string) bool { return false }

	testCases := []struct {
		username                    string
		password                    string
		validateFunc                func(username, password string) bool
		validateFuncWithDatasources func(c *container.Container, username, password string) bool
		users                       map[string]string
		success                     bool
	}{
		{
			username:                    "storedUser",
			password:                    "storedPass",
			validateFunc:                validateFuncPass,
			validateFuncWithDatasources: validateFuncDataStoreFail,
			users:                       users,
			success:                     false,
		},
		{
			username:                    "storedUser",
			password:                    "storedPass",
			validateFunc:                validateFuncFail,
			validateFuncWithDatasources: validateFuncDataStorePass,
			users:                       users,
			success:                     true,
		},
		{
			username:                    "storedUser",
			password:                    "storedPass",
			validateFunc:                validateFuncPass,
			validateFuncWithDatasources: nil,
			users:                       users,
			success:                     true,
		},
		{
			username:                    "storedUser",
			password:                    "storedPass",
			validateFunc:                validateFuncFail,
			validateFuncWithDatasources: nil,
			users:                       users,
			success:                     false,
		},
		{
			username: "storedUser",
			password: "storedPass",
			users:    users,
			success:  true,
		},
		{
			username: "storedUser",
			password: "wrongPass",
			users:    users,
			success:  false,
		},
		{
			username: "newUser",
			password: "",
			users:    users,
			success:  true,
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test Case #%d", i), func(t *testing.T) {
			provider := BasicAuthProvider{
				Users:                       tc.users,
				ValidateFunc:                tc.validateFunc,
				ValidateFuncWithDatasources: tc.validateFuncWithDatasources,
				Container:                   nil,
			}
			success := provider.validateCredentials(tc.username, tc.password)
			assert.Equal(t, tc.success, success)
		})
	}
}
