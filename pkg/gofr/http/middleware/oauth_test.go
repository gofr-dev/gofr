package middleware

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/service"
)

func TestOAuthProvider_extractAuthHeader(t *testing.T) {
	regex := regexp.MustCompile(jwtRegexPattern)

	validHeader := `Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.` +
		`eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.` +
		`KMUFsIDTnFmyG3nMiGM6H9FNFUROf3wh7SmqJp-QV30`

	testCases := []struct {
		publicKeyFunc func(token *jwt.Token) (any, error) // public key matching, not matching
		options       []jwt.ParserOption
		header        string // missing, malformed, valid
		response      any
		err           ErrorHTTP
	}{
		{
			err: ErrorMissingAuthHeader{key: headerAuthorization},
		},
		{
			header: "Bearer some-value",
			err:    ErrorInvalidAuthorizationHeaderFormat{key: headerAuthorization, errMessage: "jwt expected"},
		},
		{
			header: "Bearer a.b.c",
			err:    ErrorInvalidAuthorizationHeaderFormat{key: headerAuthorization, errMessage: "token is malformed"},
		},
		{
			publicKeyFunc: notFoundPublicKeyFunc(),
			header:        validHeader,
			err:           ErrorInvalidAuthorizationHeader{key: headerAuthorization},
		},
		{
			publicKeyFunc: emptyProviderPublicKeyFunc(),
			header:        validHeader,
			err:           ErrorInvalidConfiguration{message: "jwks configuration issue"},
		},
		{
			publicKeyFunc: validPublicKeyFunc(),
			header:        validHeader,
			response:      jwt.MapClaims{"admin": true, "iat": 1.516239022e+09, "name": "John Doe", "sub": "1234567890"},
		},
		{
			publicKeyFunc: validPublicKeyFunc(),
			header:        validHeader,
			options:       []jwt.ParserOption{jwt.WithExpirationRequired()},
			err:           ErrorInvalidAuthorizationHeader{key: headerAuthorization},
		},
	}
	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Header.Set(headerAuthorization, tc.header)
			provider := &OAuthProvider{
				publicKeyFunc: tc.publicKeyFunc,
				options:       tc.options,
				regex:         regex,
			}
			response, err := provider.ExtractAuthHeader(req)
			assert.Equal(t, tc.response, response)
			assert.Equal(t, tc.err, err)
		})
	}
}

func Test_NewOAuthProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testCases := []struct {
		jwks            JWKSProvider
		interval        time.Duration
		options         []jwt.ParserOption
		expectedOptions int
		err             error
	}{
		{err: errEmptyProvider},
		{interval: 10 * time.Second, err: errEmptyProvider},
		{jwks: service.NewMockHTTP(ctrl), interval: 0 * time.Second, err: errInvalidInterval},
		{jwks: service.NewMockHTTP(ctrl), interval: -2 * time.Second, err: errInvalidInterval},
		{jwks: service.NewMockHTTP(ctrl), interval: 10, err: errInvalidInterval},
		{jwks: service.NewMockHTTP(ctrl), interval: 10 * time.Second, expectedOptions: 1},
	}

	for i, testCase := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			config := OauthConfigs{
				Provider:        testCase.jwks,
				RefreshInterval: testCase.interval,
				Path:            "/.well-known/jwks.json", // Set a default path
			}

			// Set up mock expectations for successful cases
			if testCase.err == nil && testCase.jwks != nil {
				mockHTTP := testCase.jwks.(*service.MockHTTP)
				mockHTTP.EXPECT().GetWithHeaders(gomock.Any(), "/.well-known/jwks.json", nil, nil).
					Return(&http.Response{
						StatusCode: http.StatusOK,
						Body:       http.NoBody,
					}, nil).AnyTimes()
			}

			response, err := NewOAuthProvider(config, testCase.options...)
			assert.Equal(t, testCase.err, err)

			if testCase.err != nil {
				return
			}

			oAuthProvider, ok := response.(*OAuthProvider)

			require.True(t, ok)
			assert.NotNil(t, oAuthProvider.publicKeyFunc)
			assert.Len(t, oAuthProvider.options, testCase.expectedOptions)
		})
	}
}

func TestOAuthProvider_getAuthMethod(t *testing.T) {
	assert.Equal(t, JWTClaim, (&OAuthProvider{}).GetAuthMethod())
}

func TestJSONWebKey_rsaPublicKey(t *testing.T) {
	testCases := []struct {
		modulus        string
		publicExponent string
		response       *rsa.PublicKey
		err            error
	}{
		{err: errEmptyModulus},
		{modulus: `AQAB`, err: errEmptyPublicExponent},
		{modulus: `jw=`, publicExponent: `lorem-ipsum`, err: base64.CorruptInputError(2)},
		{modulus: `AQAB`, publicExponent: `jw====`, err: base64.CorruptInputError(2)},
		{modulus: `AQAB`, publicExponent: `AQAB`, response: &rsa.PublicKey{N: big.NewInt(65537), E: 65537}},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			key := &JSONWebKey{Modulus: tc.modulus, PublicExponent: tc.publicExponent}
			response, err := key.rsaPublicKey()
			assert.Equal(t, tc.response, response)
			assert.Equal(t, tc.err, err)
		})
	}
}

func Test_publicKeyFromJWKS(t *testing.T) {
	validJWKS := JWKS{Keys: []JSONWebKey{
		{ID: "id-1", Modulus: `AQAB`, PublicExponent: `AQAB`},
		{ID: "id-2", Modulus: `AQAB`, PublicExponent: `AQAB`},
	}}
	emptyJWKS := JWKS{}
	partialValidJWKS := JWKS{Keys: []JSONWebKey{
		{ID: "id-1", Modulus: `AQAB`, PublicExponent: `AQAB`},
		{ID: "id-2", Modulus: `AQAB`, PublicExponent: `AQAB`},
		{ID: "id-2", Modulus: ``, PublicExponent: `AQAB`},
	}}
	response := map[string]*rsa.PublicKey{
		"id-1": {N: big.NewInt(65537), E: 65537},
		"id-2": {N: big.NewInt(65537), E: 65537},
	}

	testCases := []struct {
		jwks     JWKS
		response map[string]*rsa.PublicKey
	}{
		{jwks: emptyJWKS},
		{jwks: validJWKS, response: response},
		{jwks: partialValidJWKS, response: response},
	}
	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			result := publicKeyFromJWKS(tc.jwks)
			assert.Equal(t, tc.response, result)
		})
	}
}

func TestJWKNotFound_Error(t *testing.T) {
	assert.Equal(t, "JWKS Not Found", JWKNotFound{}.Error())
}

func TestPublicKeys_Get(t *testing.T) {
	keySet := map[string]*rsa.PublicKey{
		"id-1": {N: nil, E: 0},
		"id-2": {N: nil, E: 1},
		"id-3": {N: nil, E: 2},
	}

	testCases := []struct {
		keys     map[string]*rsa.PublicKey
		keyID    string
		response *rsa.PublicKey
	}{
		{keys: keySet, keyID: "id-1", response: &rsa.PublicKey{E: 0}},
		{keys: keySet, keyID: "id-2", response: &rsa.PublicKey{E: 1}},
		{keyID: "id-1"},
		{keys: keySet, keyID: "id-0"},
	}
	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			publicKeys := PublicKeys{keys: tc.keys}
			response := publicKeys.Get(tc.keyID)
			assert.Equal(t, tc.response, response)
		})
	}
}

func Test_getPublicKeys(t *testing.T) {
	testCases := []struct {
		path           string
		responseLength int
		err            error
	}{
		{path: "/empty-body", err: errEmptyResponseBody},
		{path: "/dns-error", err: &net.DNSError{}},
		{path: "/wrong-path", err: errInvalidURL},
		{path: "/.well-known/unparseable-json", err: &json.SyntaxError{}},
		{path: "/.well-known/format-error", err: &json.UnmarshalTypeError{}},
		{path: "/empty-list"},
		{path: "", responseLength: 2},
	}
	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			response, err := getPublicKeys(t.Context(), MockJWKSProvider{}, tc.path)
			assert.Len(t, response.Keys, tc.responseLength)

			if tc.err != nil {
				assert.ErrorIs(t, err, tc.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPublicKeys_updateKeys(t *testing.T) {
	testCases := []struct {
		keys          map[string]*rsa.PublicKey
		path          string
		updatedLength int
	}{
		{keys: nil, path: "/empty-response"},
		{keys: nil, path: "/empty-list", updatedLength: 0},
		{keys: nil, path: "", updatedLength: 2},
		{keys: map[string]*rsa.PublicKey{"11": {}}, path: "/empty-response", updatedLength: 1},
		{keys: map[string]*rsa.PublicKey{"11": {}}, path: "/empty-list", updatedLength: 1},
		{keys: map[string]*rsa.PublicKey{"11": {}}, path: "", updatedLength: 2},
	}
	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			config := OauthConfigs{Provider: MockJWKSProvider{}, Path: tc.path}
			publicKeys := PublicKeys{keys: tc.keys}
			publicKeys.updateKeys(config)
			assert.Len(t, publicKeys.keys, tc.updatedLength)
		})
	}
}

func Test_getPublicKeyFunc(t *testing.T) {
	testCases := []struct {
		provider PublicKeyProvider
		response any
		err      error
		funcErr  error
	}{
		{err: errEmptyProvider},
		{provider: validPublicKeyProvider{}, response: &rsa.PublicKey{N: big.NewInt(65537), E: 65537}},
		{provider: emptyPublicKeyProvider{}, response: nil, funcErr: JWKNotFound{}},
	}
	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			function, err := getPublicKeyFunc(tc.provider)
			assert.Equal(t, tc.err, err)

			if function == nil {
				return
			}

			// Create a token with a kid header for the emptyPublicKeyProvider test
			token := &jwt.Token{}
			if i == 2 { // Test case 2 is emptyPublicKeyProvider
				token.Header = map[string]any{"kid": "test-key-id"}
			}

			response, err := function(token)
			assert.Equal(t, tc.response, response)

			if tc.funcErr != nil {
				assert.Equal(t, tc.funcErr, err)
			} else {
				assert.Equal(t, tc.err, err)
			}
		})
	}
}

func notFoundPublicKeyFunc() func(token *jwt.Token) (any, error) {
	return func(_ *jwt.Token) (any, error) {
		return nil, JWKNotFound{}
	}
}

func validPublicKeyFunc() func(token *jwt.Token) (any, error) {
	return func(_ *jwt.Token) (any, error) {
		return []byte("a-string-secret-at-least-256-bits-long"), nil
	}
}

func emptyProviderPublicKeyFunc() func(token *jwt.Token) (any, error) {
	return func(_ *jwt.Token) (any, error) {
		return nil, errEmptyProvider
	}
}

type validPublicKeyProvider struct {
}

func (validPublicKeyProvider) Get(_ string) *rsa.PublicKey {
	return &rsa.PublicKey{N: big.NewInt(65537), E: 65537}
}

type emptyPublicKeyProvider struct{}

func (emptyPublicKeyProvider) Get(_ string) *rsa.PublicKey {
	return nil
}

type MockJWKSProvider struct {
}

func (MockJWKSProvider) GetWithHeaders(_ context.Context, path string, _ map[string]any,
	_ map[string]string) (*http.Response, error) {
	keys := []JSONWebKey{{ID: "111", Type: "RSA", Modulus: "someBase64UrlEncodedModulus", PublicExponent: "AQAB"},
		{ID: "212", Type: "RSA", Modulus: "AnotherModulus", PublicExponent: "AQAB"}}

	switch path {
	case "":
		jwks := JWKS{
			Keys: keys,
		}
		jwksJSON, _ := json.Marshal(jwks)

		return &http.Response{
			Body:       io.NopCloser(bytes.NewBuffer(jwksJSON)),
			StatusCode: http.StatusOK,
		}, nil
	case "/empty-list":
		jwks := JWKS{}
		jwksJSON, _ := json.Marshal(jwks)

		return &http.Response{
			Body:       io.NopCloser(bytes.NewBuffer(jwksJSON)),
			StatusCode: http.StatusOK,
		}, nil

	case "/.well-known/format-error":
		jwksJSON, _ := json.Marshal(keys)

		return &http.Response{
			Body:       io.NopCloser(bytes.NewBuffer(jwksJSON)),
			StatusCode: http.StatusOK,
		}, nil
	case "/.well-known/unparseable-json":
		return &http.Response{
			Body:       io.NopCloser(bytes.NewBufferString(`{ "key": "value", invalid }`)),
			StatusCode: http.StatusOK,
		}, nil
	case "/wrong-path":
		return &http.Response{StatusCode: http.StatusNotFound}, nil
	case "/dns-error":
		return nil, &net.DNSError{}
	default:
		return &http.Response{StatusCode: http.StatusOK}, nil
	}
}
