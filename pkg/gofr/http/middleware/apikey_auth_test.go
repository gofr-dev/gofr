package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/container"
)

const (
	validKey1 string = "valid-key-1"
	validKey2 string = "valid-key-2"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func Test_NewAPIKeyAuthProvider(t *testing.T) {
	testCases := []struct {
		apiKeys  []string
		provider AuthProvider
		err      error
	}{
		{err: errAPIKeyEmpty},
		{apiKeys: make([]string, 0), err: errAPIKeyEmpty},
		{apiKeys: []string{validKey1}, provider: &APIKeyAuthProvider{APIKeys: []string{validKey1}}, err: nil},
	}
	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			provider, err := NewAPIKeyAuthProvider(tc.apiKeys)
			assert.Equal(t, tc.provider, provider)
			assert.Equal(t, tc.err, err)
		})
	}
}

func Test_NewAPIKeyAuthProviderWithValidateFunc(t *testing.T) {
	validateFunc := func(_ *container.Container, _ string) bool {
		return true
	}
	c := container.Container{}
	provider := APIKeyAuthProvider{
		ValidateFuncWithDatasources: validateFunc,
		Container:                   &c,
	}
	testCases := []struct {
		validateFunc func(*container.Container, string) bool
		container    *container.Container
		provider     AuthProvider
		err          error
	}{
		{err: errContainerNil},
		{validateFunc: validateFunc, err: errContainerNil},
		{validateFunc: validateFunc, container: &c, provider: &provider},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			authProvider, err := NewAPIKeyAuthProviderWithValidateFunc(tc.container, tc.validateFunc)
			assert.Equal(t, tc.err, err)

			if err != nil {
				return
			}

			apiAuthProvider, ok := authProvider.(*APIKeyAuthProvider)
			require.True(t, ok)
			expected, ok := tc.provider.(*APIKeyAuthProvider)
			require.True(t, ok)
			assert.Equal(t, expected.Container, apiAuthProvider.Container)
			assert.NotNil(t, apiAuthProvider.ValidateFuncWithDatasources)
			assert.Nil(t, apiAuthProvider.ValidateFunc)
			assert.Empty(t, apiAuthProvider.APIKeys)
		})
	}
}

func Test_extractAuthHeader(t *testing.T) {
	provider := APIKeyAuthProvider{APIKeys: []string{validKey1, validKey2}}
	testCases := []struct {
		header   string
		err      ErrorHTTP
		response any
	}{
		{err: ErrorMissingAuthHeader{key: headerXAPIKey}},
		{header: "some-value", err: ErrorInvalidAuthorizationHeader{key: headerXAPIKey}},
		{header: validKey1, response: validKey1},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Header.Set(headerXAPIKey, tc.header)
			response, err := provider.ExtractAuthHeader(req)
			assert.Equal(t, tc.response, response)
			assert.Equal(t, tc.err, err)
		})
	}
}

func Test_authMethod(t *testing.T) {
	authProvider := APIKeyAuthProvider{}
	assert.Equal(t, APIKey, authProvider.GetAuthMethod())
}

func Test_validateAPIKey(t *testing.T) {
	validateFuncSuccess := func(_ string) bool {
		return true
	}
	validateFuncFail := func(_ string) bool {
		return false
	}

	validateFuncDataSourcesPass := func(_ *container.Container, _ string) bool {
		return true
	}
	validateFuncDataSourcesFail := func(_ *container.Container, _ string) bool {
		return false
	}

	apiKeys := []string{validKey1, validKey2}

	testCases := []struct {
		validateFuncDS func(*container.Container, string) bool // 3 possible values
		validateFunc   func(string) bool                       // 3 possible values
		apiKeys        []string                                // 2 possible values
		apiKey         string                                  // 2 possible values
		result         bool
	}{
		{validateFuncDS: validateFuncDataSourcesPass, validateFunc: validateFuncFail, apiKeys: apiKeys, apiKey: validKey1, result: true},
		{validateFuncDS: validateFuncDataSourcesPass, validateFunc: validateFuncFail, apiKey: validKey1, result: true},
		{validateFuncDS: validateFuncDataSourcesPass, validateFunc: validateFuncSuccess, apiKeys: apiKeys, apiKey: validKey1, result: true},
		{validateFuncDS: validateFuncDataSourcesPass, validateFunc: validateFuncSuccess, apiKey: validKey1, result: true},
		{validateFuncDS: validateFuncDataSourcesPass, apiKeys: apiKeys, apiKey: validKey1, result: true},
		{validateFuncDS: validateFuncDataSourcesPass, apiKey: validKey1, result: true},

		{validateFuncDS: validateFuncDataSourcesFail, validateFunc: validateFuncFail, apiKeys: apiKeys, apiKey: validKey1, result: false},
		{validateFuncDS: validateFuncDataSourcesFail, validateFunc: validateFuncFail, apiKey: validKey1, result: false},
		{validateFuncDS: validateFuncDataSourcesFail, validateFunc: validateFuncSuccess, apiKeys: apiKeys, apiKey: validKey1, result: false},
		{validateFuncDS: validateFuncDataSourcesFail, validateFunc: validateFuncSuccess, apiKey: validKey1, result: false},
		{validateFuncDS: validateFuncDataSourcesFail, apiKeys: apiKeys, apiKey: validKey1, result: false},
		{validateFuncDS: validateFuncDataSourcesFail, apiKey: validKey1, result: false},

		{validateFunc: validateFuncSuccess, apiKeys: apiKeys, apiKey: validKey1, result: true},
		{validateFunc: validateFuncFail, apiKeys: apiKeys, apiKey: validKey1, result: false},
		{validateFunc: validateFuncSuccess, apiKey: validKey1, result: true},
		{validateFunc: validateFuncFail, apiKey: validKey1, result: false},

		{apiKeys: apiKeys, apiKey: validKey1, result: true},
		{apiKeys: apiKeys, apiKey: "wrong-key", result: false},
		{apiKey: validKey1, result: false},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			authProvider := APIKeyAuthProvider{
				ValidateFunc:                tc.validateFunc,
				ValidateFuncWithDatasources: tc.validateFuncDS,
				APIKeys:                     tc.apiKeys,
			}
			result := authProvider.validateAPIKey(tc.apiKey)
			assert.Equal(t, tc.result, result)
		})
	}
}

func Test_APIKeyAuthMiddleware(t *testing.T) {
	t.Logf("Test_APIKeyAuthMiddleware")
}
