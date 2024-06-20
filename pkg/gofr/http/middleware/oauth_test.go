package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func TestOAuthSuccess(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet).Name("/test")
	router.Use(OAuth(NewOAuth(OauthConfigs{Provider: &MockProvider{}, RefreshInterval: 10})))

	server := httptest.NewServer(router)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6IjAwVFEwdlRpNVB1UnZscUZGY3dCeUc0WjBM"+
		"dGREcUtJX0JWUFRrdnpleEUiLCJ0eXAiOiJKV1QifQ.eyJhdWQiOiJzdGFnZS5rb3BzLmRldiIsImlhdCI6MTI1Nzg5NDAwMCwib3JpZyI6IkdP"+
		"T0dMRSIsInBpY3R1cmUiOiJodHRwczovL2xoMy5nb29nbGV1c2VyY29udGVudC5jb20vYS9BQ2c4b2NLSjVEREE0enJ1ekZsc1E5S3ZMakhEdG"+
		"JPVF9ocFZ6MGhFTzhqU2wybTdNeWs9czk2LWMiLCJzdWIiOiJyYWtzaGl0LnNpbmdoQHpvcHNtYXJ0LmNvbSIsInN1Yi1pZCI6ImE2NTczZTFkL"+
		"WFiZWEtNDg2My1hY2RiLTZjZjM2MjZhNDQxNCIsInR5cCI6InJlZnJlc2hfdG9rZW4ifQ.NkYSi6KJtGA3js9dcN3UqJWfeJdB88p7cxclrc6"+
		"fxJODlCalsbbwIr3QL4AR9i0ucJjmoTIipCwpdM1IYDjCd-ilf2mTp11Wba31XoH--8YLI9Ju0wbpYhtF3wa00NF1Ijt48ze09IJ6QtE-etm"+
		"AN8T7izsXbPeSrFiN3NVQU87eGxc3bEQhEsV5u3E6j8EdVDv8xbwisETY-N0mDftZp0w8UCkQ7MarOrA5IaXs2MHyCETy5y9QFd4djppH9oFo"+
		"y5-AtEZqzyHKfGMlerjtJp8uOgFso9FycGuO0TFhR4AaZGVZxB072Hu-71tbx7atXp3zmDdkK_jkg5aVepoU_Q")

	client := http.Client{}

	resp, err := client.Do(req)

	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp.Body.Close()
}

func TestOAuthInvalidTokenFormat(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet).Name("/test")
	router.Use(OAuth(NewOAuth(OauthConfigs{Provider: &MockProvider{}, RefreshInterval: 10})))

	server := httptest.NewServer(router)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", http.NoBody)
	req.Header.Set("Authorization", "eyJhb")

	client := http.Client{}

	resp, err := client.Do(req)

	respBody, _ := io.ReadAll(resp.Body)

	assert.Nil(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t, string(respBody), `Authorization header format must be Bearer {token}`)

	resp.Body.Close()
}

func TestOAuthEmptyAuthHeader(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet).Name("/test")
	router.Use(OAuth(NewOAuth(OauthConfigs{Provider: &MockProvider{}, RefreshInterval: 10})))

	server := httptest.NewServer(router)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", http.NoBody)

	client := http.Client{}

	resp, err := client.Do(req)

	respBody, _ := io.ReadAll(resp.Body)

	assert.Nil(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t, string(respBody), `Authorization header is required`)

	resp.Body.Close()
}

func TestOAuthMalformedToken(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet).Name("/test")
	router.Use(OAuth(NewOAuth(OauthConfigs{Provider: &MockProvider{}, RefreshInterval: 10})))

	server := httptest.NewServer(router)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer eyJh")

	client := http.Client{}

	resp, err := client.Do(req)

	respBody, _ := io.ReadAll(resp.Body)

	assert.Nil(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t, string(respBody), `token is malformed: token contains an invalid number of segments`)

	resp.Body.Close()
}

func TestOAuthJWKSKeyNotFound(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet).Name("/test")
	router.Use(OAuth(NewOAuth(OauthConfigs{Provider: &MockProvider{}, RefreshInterval: 10})))

	server := httptest.NewServer(router)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6IjhaOV9RQTBSa0Y3RHM3TUFNaDFxLTl6dVJZ"+
		"TklSTThHV3BZdXdyb0ZkTjg9IiwidHlwIjoiSldUIn0.eyJhdWQiOiJzdGFnZS5hdXRoLnpvcHNtYXJ0LmNvbSIsImV4cCI6MTcxODc5MDQ2My"+
		"wiaWF0IjoxNzEwMTUwNDYzLCJpc3MiOiJzdGFnZS5hdXRoLnpvcHNtYXJ0LmNvbSIsIm5hbWUiOiJSYWtzaGl0IFNpbmdoIiwib3JpZyI6IkdP"+
		"T0dMRSIsInBpY3R1cmUiOiJodHRwczovL2xoMy5nb29nbGV1c2VyY29udGVudC5jb20vYS9BQ2c4b2NLSjVEREE0enJ1ekZsc1E5S3ZMakhEdG"+
		"JPVF9ocFZ6MGhFTzhqU2wybTdNeWs9czk2LWMiLCJzdWIiOiJyYWtzaGl0LnNpbmdoQHpvcHNtYXJ0LmNvbSIsInN1Yi1pZCI6ImE2NTczZTFk"+
		"LWFiZWEtNDg2My1hY2RiLTZjZjM2MjZhNDQxNCIsInR5cCI6InJlZnJlc2hfdG9rZW4ifQ.SYs0UY1uCYly1mAHmr5KLUgdze8dXX5Ee4dueL"+
		"br4wo4sjucmG1uyprheGhLbc5frwIMxHjliIToHgTzyOYeyJNnBbyihnoNjHEFgEU-Sy_-mPXLP6cUkEJKf4SzDroGDNLoYqJb_wZglqrTxFt81"+
		"bO3itEsp3puK-u_Y0VL9Mu2kKZJDY9sRAxI39inKIu-S1A14nHaXuGox9FHAfRv6Vs7Pk2RloNa3C6NB8mCNeg40sP1G-hgUlJMmYG0q6DJL9N"+
		"xOvpVZk_Trs01pfkXqpyoI4Q2GzuvjlByidxX-XeWLjd8YfuPA5IDyYiKPf8pqvqa47I1yXky0o_eXmnvDw")

	client := http.Client{}

	resp, err := client.Do(req)

	respBody, _ := io.ReadAll(resp.Body)

	assert.Nil(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t, string(respBody), `token is unverifiable: error while executing keyfunc`)

	resp.Body.Close()
}

func TestPublicKeyFromJWKS_EmptyJWKS_ReturnsNil(t *testing.T) {
	jwks := JWKS{}

	result := publicKeyFromJWKS(jwks)

	assert.Nil(t, result)
}

func Test_OAuth_well_known(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/.well-known/health-check", http.NoBody)
	rr := httptest.NewRecorder()

	authMiddleware := OAuth(nil)(testHandler)
	authMiddleware.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code, "TEST Failed.\n")

	assert.Equal(t, "Success", rr.Body.String(), "TEST Failed.\n")
}

func TestOAuthHTTPCallFailed(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet).Name("/test")
	router.Use(OAuth(NewOAuth(OauthConfigs{Provider: &MockErrorProvider{}, RefreshInterval: 10})))

	server := httptest.NewServer(router)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6IklDbmFZdEwtSDExckl0WlJ4VVlLVElzbm"+
		"5ybm1wWUp6cGFWRHVDRWN0Ukk9IiwidHlwIjoiSldUIn0.eyJhdWQiOiJzdGFnZS5rb3BzLmRldiIsImV4cCI6MTcxODc5MjQ2NiwiaWF0Ij"+
		"oxNzEwMTUyNDY2LCJpc3MiOiJzdGFnZS5hdXRoLnpvcHNtYXJ0LmNvbSIsIm5hbWUiOiJSYWtzaGl0IFNpbmdoIiwib3JpZyI6IkdPT0dMRSI"+
		"sInBpY3R1cmUiOiJodHRwczovL2xoMy5nb29nbGV1c2VyY29udGVudC5jb20vYS9BQ2c4b2NLSjVEREE0enJ1ekZsc1E5S3ZMakhEdGJPVF9o"+
		"cFZ6MGhFTzhqU2wybTdNeWs9czk2LWMiLCJzdWIiOiJyYWtzaGl0LnNpbmdoQHpvcHNtYXJ0LmNvbSIsInN1Yi1pZCI6ImE2NTczZTFkLWFiZW"+
		"EtNDg2My1hY2RiLTZjZjM2MjZhNDQxNCIsInR5cCI6InJlZnJlc2hfdG9rZW4ifQ.eoRVSFcyvbWk-fUSlACI4pWwHcuwjA1BbKlYA_aEJA6T"+
		"BRcnM0HoaxL_GcF0Q-95Z6Medk9l5Fe-zuY4xmLX0XRnA9y9KEsXvyhxsmLJTV32C2kirDh6TR5FIep3EKV0VdWKJT6LziBjrCP-F0pKb34em"+
		"Ua7gsyi5OnkX12_ZcGpQpSbL3mcZpEEGUmKijlg1VspK4G9dTmNSUXofxStokxacLwa3hiFfkd7vtegkF79bfWPVm0hlJDGDcU7szUaIyHjdW"+
		"rlUGqQ0A8-8dYQ-Z1o5STZITcxvSv6SaZNo08r_szi-TDLXRhASP3ojEjFCqFBmPw9HPxHG4JmV3SX2A")

	client := http.Client{}

	resp, err := client.Do(req)

	assert.Nil(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	resp.Body.Close()
}

func TestOAuthReadError(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet).Name("/test")
	router.Use(OAuth(NewOAuth(OauthConfigs{Provider: &MockReaderErrorProvider{}, RefreshInterval: 10})))

	server := httptest.NewServer(router)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6IklDbmFZdEwtSDExckl0WlJ4VVlLVElzbm"+
		"5ybm1wWUp6cGFWRHVDRWN0Ukk9IiwidHlwIjoiSldUIn0.eyJhdWQiOiJzdGFnZS5rb3BzLmRldiIsImV4cCI6MTcxODc5MjQ2NiwiaWF0Ij"+
		"oxNzEwMTUyNDY2LCJpc3MiOiJzdGFnZS5hdXRoLnpvcHNtYXJ0LmNvbSIsIm5hbWUiOiJSYWtzaGl0IFNpbmdoIiwib3JpZyI6IkdPT0dMRSI"+
		"sInBpY3R1cmUiOiJodHRwczovL2xoMy5nb29nbGV1c2VyY29udGVudC5jb20vYS9BQ2c4b2NLSjVEREE0enJ1ekZsc1E5S3ZMakhEdGJPVF9o"+
		"cFZ6MGhFTzhqU2wybTdNeWs9czk2LWMiLCJzdWIiOiJyYWtzaGl0LnNpbmdoQHpvcHNtYXJ0LmNvbSIsInN1Yi1pZCI6ImE2NTczZTFkLWFiZW"+
		"EtNDg2My1hY2RiLTZjZjM2MjZhNDQxNCIsInR5cCI6InJlZnJlc2hfdG9rZW4ifQ.eoRVSFcyvbWk-fUSlACI4pWwHcuwjA1BbKlYA_aEJA6T"+
		"BRcnM0HoaxL_GcF0Q-95Z6Medk9l5Fe-zuY4xmLX0XRnA9y9KEsXvyhxsmLJTV32C2kirDh6TR5FIep3EKV0VdWKJT6LziBjrCP-F0pKb34em"+
		"Ua7gsyi5OnkX12_ZcGpQpSbL3mcZpEEGUmKijlg1VspK4G9dTmNSUXofxStokxacLwa3hiFfkd7vtegkF79bfWPVm0hlJDGDcU7szUaIyHjdW"+
		"rlUGqQ0A8-8dYQ-Z1o5STZITcxvSv6SaZNo08r_szi-TDLXRhASP3ojEjFCqFBmPw9HPxHG4JmV3SX2A")

	client := http.Client{}

	resp, err := client.Do(req)

	assert.Nil(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	resp.Body.Close()
}

func TestOAuthJSONUnmarshalError(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet).Name("/test")
	router.Use(OAuth(NewOAuth(OauthConfigs{Provider: &MockJSONResponseErrorProvider{}, RefreshInterval: 10})))

	server := httptest.NewServer(router)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6IklDbmFZdEwtSDExckl0WlJ4VVlLVElzbm"+
		"5ybm1wWUp6cGFWRHVDRWN0Ukk9IiwidHlwIjoiSldUIn0.eyJhdWQiOiJzdGFnZS5rb3BzLmRldiIsImV4cCI6MTcxODc5MjQ2NiwiaWF0Ij"+
		"oxNzEwMTUyNDY2LCJpc3MiOiJzdGFnZS5hdXRoLnpvcHNtYXJ0LmNvbSIsIm5hbWUiOiJSYWtzaGl0IFNpbmdoIiwib3JpZyI6IkdPT0dMRSI"+
		"sInBpY3R1cmUiOiJodHRwczovL2xoMy5nb29nbGV1c2VyY29udGVudC5jb20vYS9BQ2c4b2NLSjVEREE0enJ1ekZsc1E5S3ZMakhEdGJPVF9o"+
		"cFZ6MGhFTzhqU2wybTdNeWs9czk2LWMiLCJzdWIiOiJyYWtzaGl0LnNpbmdoQHpvcHNtYXJ0LmNvbSIsInN1Yi1pZCI6ImE2NTczZTFkLWFiZW"+
		"EtNDg2My1hY2RiLTZjZjM2MjZhNDQxNCIsInR5cCI6InJlZnJlc2hfdG9rZW4ifQ.eoRVSFcyvbWk-fUSlACI4pWwHcuwjA1BbKlYA_aEJA6T"+
		"BRcnM0HoaxL_GcF0Q-95Z6Medk9l5Fe-zuY4xmLX0XRnA9y9KEsXvyhxsmLJTV32C2kirDh6TR5FIep3EKV0VdWKJT6LziBjrCP-F0pKb34em"+
		"Ua7gsyi5OnkX12_ZcGpQpSbL3mcZpEEGUmKijlg1VspK4G9dTmNSUXofxStokxacLwa3hiFfkd7vtegkF79bfWPVm0hlJDGDcU7szUaIyHjdW"+
		"rlUGqQ0A8-8dYQ-Z1o5STZITcxvSv6SaZNo08r_szi-TDLXRhASP3ojEjFCqFBmPw9HPxHG4JmV3SX2A")

	client := http.Client{}

	resp, err := client.Do(req)

	assert.Nil(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	resp.Body.Close()
}

type MockProvider struct {
}

func (m *MockProvider) GetWithHeaders(context.Context, string, map[string]interface{},
	map[string]string) (*http.Response, error) {
	// Marshal the JSON body
	responseBody := map[string]interface{}{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"use": "sig",
				"kid": "00TQ0vTi5PuRvlqFFcwByG4Z0LtdDqKI_BVPTkvzexE",
				"n": "0nb5fKw3xb4_NMkEh80jG0_HuKByBnTIRqPTX-xbtSEDTsev1O4oyl3az0UdebyimwqHSLPVFIitHnfhHsto0IycnL9omEm" +
					"40YWEUxOqs5HJaFhZsKHZmxCUkYsb-nHhYm67sYiPkcBQrisWFJi4r48EyLv050D85MkhPiD3Iy0Q5m29U-Hf9CIfxy1MS8akJ" +
					"uTnk8Ir4ajHN7ze33IOAjE1UPX1viZ6QSwbFPo0YrGf6vZq21cbhS6UD1JC-A_iFVdSGKzBAfFspQaAllifmaym6XK-q4mKqTW" +
					"430zKlGCnQd3ddg3zmCe7KqpJ6aDVUQ0FS_K8GnOoWeScWEj0qw",
				"e": "AQAB",
			},
		},
	}

	jsonResponse, err := json.Marshal(responseBody)
	if err != nil {
		return nil, err
	}

	// Construct an HTTP response with the JSON body
	response := &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}

	response.Body = http.NoBody // Reset the response body
	response.Body = io.NopCloser(bytes.NewReader(jsonResponse))

	return response, nil
}

type MockErrorProvider struct {
}

func (m *MockErrorProvider) GetWithHeaders(context.Context, string, map[string]interface{},
	map[string]string) (*http.Response, error) {
	// Marshal the JSON body
	return nil, oauthError{msg: "response error"}
}

type oauthError struct {
	msg string
}

func (o oauthError) Error() string {
	return o.msg
}

// CustomReader simulates an error during the Read operation.
type CustomReader struct{}

func (r *CustomReader) Read([]byte) (int, error) {
	return 0, oauthError{msg: "read error"}
}

type MockReaderErrorProvider struct{}

func (m *MockReaderErrorProvider) GetWithHeaders(context.Context, string, map[string]interface{},
	map[string]string) (*http.Response, error) {
	// Create a custom reader that returns an error
	body := &CustomReader{}

	// Create an http.Response with the custom reader as the body
	response := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(body),
	}

	return response, nil
}

type MockJSONResponseErrorProvider struct{}

func (m *MockJSONResponseErrorProvider) GetWithHeaders(context.Context, string, map[string]interface{},
	map[string]string) (*http.Response, error) {
	// Create a body with invalid JSON
	body := strings.NewReader("invalid JSON")

	// Create an http.Response with the invalid JSON body
	response := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(body),
	}

	return response, nil
}
