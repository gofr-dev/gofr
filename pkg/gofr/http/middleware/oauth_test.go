package middleware

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/mock"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp.Body.Close()
}

func TestGetJwtClaims(t *testing.T) {
	claims := []byte(`{"aud":"stage.kops.dev","iat":1257894000,"orig":"GOOGLE",` +
		`"picture":"https://lh3.googleusercontent.com/a/ACg8ocKJ5DDA4zruzFlsQ9KvL` +
		`jHDtbOT_hpVz0hEO8jSl2m7Myk=s96-c","sub":"rakshit.singh@zopsmart.com","sub-id"` +
		`:"a6573e1d-abea-4863-acdb-6cf3626a4414","typ":"refresh_token"}`)

	router := mux.NewRouter()
	router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		result, err := json.Marshal(r.Context().Value(JWTClaim))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_, err = w.Write(result)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

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
	result := make([]byte, len(claims))
	_, _ = resp.Body.Read(result)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, claims, result)

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

	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t, string(respBody), `authorization header format must be Bearer {token}`)

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

	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t, string(respBody), `authorization header is required`)

	resp.Body.Close()
}

func TestOAuthMalformedToken(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet).Name("/test")
	router.Use(OAuth(NewOAuth(OauthConfigs{Provider: &MockProvider{}, RefreshInterval: 1 * time.Millisecond})))

	server := httptest.NewServer(router)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer eyJh")

	client := http.Client{}

	resp, err := client.Do(req)

	respBody, _ := io.ReadAll(resp.Body)

	require.NoError(t, err)
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

	require.NoError(t, err)
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

	require.NoError(t, err)
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

	require.NoError(t, err)
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

	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	resp.Body.Close()
}

type MockProvider struct {
}

func (*MockProvider) GetWithHeaders(context.Context, string, map[string]any,
	map[string]string) (*http.Response, error) {
	// Marshal the JSON body
	responseBody := map[string]any{
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

func (*MockErrorProvider) GetWithHeaders(context.Context, string, map[string]any,
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

func (*CustomReader) Read([]byte) (int, error) {
	return 0, oauthError{msg: "read error"}
}

type MockReaderErrorProvider struct{}

func (*MockReaderErrorProvider) GetWithHeaders(context.Context, string, map[string]any,
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

func (*MockJSONResponseErrorProvider) GetWithHeaders(context.Context, string, map[string]any,
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

func Test_validateIssuer(t *testing.T) {
	tests := []struct {
		name        string
		claims      jwt.MapClaims
		config      ClaimConfig
		expectedErr error
	}{
		{
			name: "valid issuer",
			claims: jwt.MapClaims{
				"iss": "trusted-issuer",
			},
			config:      ClaimConfig{trustedIssuer: "trusted-issuer"},
			expectedErr: nil,
		},
		{
			name: "invalid issuer",
			claims: jwt.MapClaims{
				"iss": "untrusted-issuer",
			},
			config:      ClaimConfig{trustedIssuer: "trusted-issuer"},
			expectedErr: errInvalidIssuer,
		},
		{
			name:        "missing issuer claim",
			claims:      jwt.MapClaims{},
			config:      ClaimConfig{trustedIssuer: "trusted-issuer"},
			expectedErr: errInvalidIssuer,
		},
		{
			name: "no issuer config",
			claims: jwt.MapClaims{
				"iss": "any-issuer",
			},
			config:      ClaimConfig{},
			expectedErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateIssuer(tc.claims, &tc.config)
			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_validateAudience(t *testing.T) {
	validAudiences := []string{"aud1", "aud2"}

	tests := []struct {
		name        string
		claims      jwt.MapClaims
		config      ClaimConfig
		expectedErr error
	}{
		{
			name: "valid string audience",
			claims: jwt.MapClaims{
				"aud": "aud1",
			},
			config:      ClaimConfig{validAudiences: validAudiences},
			expectedErr: nil,
		},
		{
			name: "valid array audience",
			claims: jwt.MapClaims{
				"aud": []any{"aud3", "aud1"},
			},
			config:      ClaimConfig{validAudiences: validAudiences},
			expectedErr: nil,
		},
		{
			name: "invalid string audience",
			claims: jwt.MapClaims{
				"aud": "aud3",
			},
			config:      ClaimConfig{validAudiences: validAudiences},
			expectedErr: errInvalidAudience,
		},
		{
			name: "invalid array audience",
			claims: jwt.MapClaims{
				"aud": []any{"aud3", "aud4"},
			},
			config:      ClaimConfig{validAudiences: validAudiences},
			expectedErr: errInvalidAudience,
		},
		{
			name: "invalid audience type",
			claims: jwt.MapClaims{
				"aud": 123,
			},
			config:      ClaimConfig{validAudiences: validAudiences},
			expectedErr: errInvalidAudience,
		},
		{
			name:        "missing audience claim",
			claims:      jwt.MapClaims{},
			config:      ClaimConfig{validAudiences: validAudiences},
			expectedErr: errInvalidAudience,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAudience(tc.claims, &tc.config)
			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_validateSubject(t *testing.T) {
	allowedSubjects := []string{"sub1", "sub2"}

	tests := []struct {
		name        string
		claims      jwt.MapClaims
		config      ClaimConfig
		expectedErr error
	}{
		{
			name: "valid string subject",
			claims: jwt.MapClaims{
				"sub": "sub1",
			},
			config:      ClaimConfig{allowedSubjects: allowedSubjects},
			expectedErr: nil,
		},
		{
			name: "valid array subject",
			claims: jwt.MapClaims{
				"sub": []any{"sub1", "sub3"},
			},
			config:      ClaimConfig{allowedSubjects: allowedSubjects},
			expectedErr: nil,
		},
		{
			name: "invalid string subject",
			claims: jwt.MapClaims{
				"sub": "sub3",
			},
			config:      ClaimConfig{allowedSubjects: allowedSubjects},
			expectedErr: errInvalidSubjects,
		},
		{
			name: "invalid array subject",
			claims: jwt.MapClaims{
				"sub": []any{"sub3", "sub4"},
			},
			config:      ClaimConfig{allowedSubjects: allowedSubjects},
			expectedErr: errInvalidSubjects,
		},
		{
			name: "invalid subject type",
			claims: jwt.MapClaims{
				"sub": 123,
			},
			config:      ClaimConfig{allowedSubjects: allowedSubjects},
			expectedErr: errInvalidSubjects,
		},
		{
			name:        "missing subject claim",
			claims:      jwt.MapClaims{},
			config:      ClaimConfig{allowedSubjects: allowedSubjects},
			expectedErr: errInvalidSubjects,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSubject(tc.claims, &tc.config)
			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_validateExpiry(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name        string
		claims      jwt.MapClaims
		config      ClaimConfig
		expectedErr error
	}{
		{
			name: "valid expiry",
			claims: jwt.MapClaims{
				"exp": float64(now + 1000),
			},
			config:      ClaimConfig{checkExpiry: true},
			expectedErr: nil,
		},
		{
			name: "expired token",
			claims: jwt.MapClaims{
				"exp": float64(now - 1000),
			},
			config:      ClaimConfig{checkExpiry: true},
			expectedErr: errTokenExpired,
		},
		{
			name:        "missing exp claim",
			claims:      jwt.MapClaims{},
			config:      ClaimConfig{checkExpiry: true},
			expectedErr: errTokenExpired,
		},
		{
			name: "expiry check disabled",
			claims: jwt.MapClaims{
				"exp": float64(now - 1000),
			},
			config:      ClaimConfig{},
			expectedErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateExpiry(tc.claims, &tc.config)
			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_validateNotBefore(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name        string
		claims      jwt.MapClaims
		config      ClaimConfig
		expectedErr error
	}{
		{
			name: "valid not before",
			claims: jwt.MapClaims{
				"nbf": float64(now - 1000),
			},
			config:      ClaimConfig{checkNotBefore: true},
			expectedErr: nil,
		},
		{
			name: "token not active",
			claims: jwt.MapClaims{
				"nbf": float64(now + 1000),
			},
			config:      ClaimConfig{checkNotBefore: true},
			expectedErr: errTokenNotActive,
		},
		{
			name:        "missing nbf claim",
			claims:      jwt.MapClaims{},
			config:      ClaimConfig{checkNotBefore: true},
			expectedErr: nil,
		},
		{
			name: "not before check disabled",
			claims: jwt.MapClaims{
				"nbf": float64(now + 1000),
			},
			config:      ClaimConfig{},
			expectedErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateNotBefore(tc.claims, &tc.config)
			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_validateIssuedAt(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	earlier := now.Add(-1 * time.Hour)
	later := now.Add(1 * time.Hour)

	tests := []struct {
		name        string
		claims      jwt.MapClaims
		config      ClaimConfig
		expectedErr string
	}{
		{
			name: "valid exact time",
			claims: jwt.MapClaims{
				"iat": float64(now.Unix()),
			},
			config: ClaimConfig{
				issuedAtRule: IssuedAtConstraint{
					enabled: true,
					exact:   &now,
				},
			},
		},
		{
			name: "invalid exact time",
			claims: jwt.MapClaims{
				"iat": float64(earlier.Unix()),
			},
			config: ClaimConfig{
				issuedAtRule: IssuedAtConstraint{
					enabled: true,
					exact:   &now,
				},
			},
			expectedErr: fmt.Sprintf("token issued at %s does not match exact required time %s",
				earlier.Format(time.RFC3339), now.Format(time.RFC3339)),
		},
		{
			name: "valid before constraint",
			claims: jwt.MapClaims{
				"iat": float64(earlier.Unix()),
			},
			config: ClaimConfig{
				issuedAtRule: IssuedAtConstraint{
					enabled: true,
					before:  &now,
				},
			},
		},
		{
			name: "invalid before constraint",
			claims: jwt.MapClaims{
				"iat": float64(now.Unix()),
			},
			config: ClaimConfig{
				issuedAtRule: IssuedAtConstraint{
					enabled: true,
					before:  &earlier,
				},
			},
			expectedErr: fmt.Sprintf("token issued at %s is not before %s",
				now.Format(time.RFC3339), earlier.Format(time.RFC3339)),
		},
		{
			name: "valid after constraint",
			claims: jwt.MapClaims{
				"iat": float64(later.Unix()),
			},
			config: ClaimConfig{
				issuedAtRule: IssuedAtConstraint{
					enabled: true,
					after:   &now,
				},
			},
		},
		{
			name: "invalid after constraint",
			claims: jwt.MapClaims{
				"iat": float64(now.Unix()),
			},
			config: ClaimConfig{
				issuedAtRule: IssuedAtConstraint{
					enabled: true,
					after:   &later,
				},
			},
			expectedErr: fmt.Sprintf("token issued at %s is not after %s",
				now.Format(time.RFC3339), later.Format(time.RFC3339)),
		},
		{
			name: "multiple constraints - exact takes precedence",
			claims: jwt.MapClaims{
				"iat": float64(now.Unix()),
			},
			config: ClaimConfig{
				issuedAtRule: IssuedAtConstraint{
					enabled: true,
					exact:   &now,
					before:  &later,
					after:   &earlier,
				},
			},
		},
		{
			name:   "missing iat claim",
			claims: jwt.MapClaims{},
			config: ClaimConfig{
				issuedAtRule: IssuedAtConstraint{enabled: true},
			},
			expectedErr: "invalid issued at time",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateIssuedAt(tc.claims, &tc.config)

			if tc.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClaimOptions(t *testing.T) {
	t.Run("WithTrustedIssuer", func(t *testing.T) {
		cfg := &ClaimConfig{}
		option := WithTrustedIssuer("trusted-issuer")
		option(cfg)

		assert.Equal(t, "trusted-issuer", cfg.trustedIssuer)
	})

	t.Run("WithValidAudiences", func(t *testing.T) {
		cfg := &ClaimConfig{}
		option := WithValidAudiences("aud1", "aud2")
		option(cfg)

		assert.ElementsMatch(t, []string{"aud1", "aud2"}, cfg.validAudiences)
	})

	t.Run("WithAllowedSubjects", func(t *testing.T) {
		cfg := &ClaimConfig{}
		option := WithAllowedSubjects("sub1", "sub2")
		option(cfg)

		assert.ElementsMatch(t, []string{"sub1", "sub2"}, cfg.allowedSubjects)
	})

	t.Run("EnforceExpiryCheck", func(t *testing.T) {
		cfg := &ClaimConfig{}
		option := EnforceExpiryCheck()
		option(cfg)

		assert.True(t, cfg.checkExpiry)
	})

	t.Run("EnforceNotBeforeCheck", func(t *testing.T) {
		cfg := &ClaimConfig{}
		option := EnforceNotBeforeCheck()
		option(cfg)

		assert.True(t, cfg.checkNotBefore)
	})

	t.Run("IssuedBefore", func(t *testing.T) {
		cfg := &ClaimConfig{}
		beforeTime := time.Now().Add(-time.Hour)
		option := IssuedBefore(beforeTime)
		option(cfg)

		assert.True(t, cfg.issuedAtRule.enabled)
		assert.NotNil(t, cfg.issuedAtRule.before)
		assert.True(t, cfg.issuedAtRule.before.Before(time.Now()))
	})

	t.Run("IssuedAfter", func(t *testing.T) {
		cfg := &ClaimConfig{}
		afterTime := time.Now().Add(time.Hour)
		option := IssuedAfter(afterTime)
		option(cfg)

		assert.True(t, cfg.issuedAtRule.enabled)
		assert.NotNil(t, cfg.issuedAtRule.after)
		assert.True(t, cfg.issuedAtRule.after.After(time.Now().Add(-time.Hour)))
	})

	t.Run("IssuedAt", func(t *testing.T) {
		cfg := &ClaimConfig{}
		exactTime := time.Now()
		option := IssuedAt(exactTime)
		option(cfg)

		assert.True(t, cfg.issuedAtRule.enabled)
		assert.NotNil(t, cfg.issuedAtRule.exact)
		assert.Equal(t, exactTime.Truncate(time.Second), *cfg.issuedAtRule.exact)
	})
}

type mockJWKSProvider struct {
	mock.Mock
}

func (m *mockJWKSProvider) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any, headers map[string]string) (*http.Response, error) {
	args := m.Called(ctx, path, queryParams, headers)
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestOAuthMiddleware(t *testing.T) {
	// Generate proper RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Create valid JWKS response with the public key
	jwks := createJWKSResponse(privateKey.PublicKey)
	mockProvider := new(mockJWKSProvider)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(jwks)),
	}
	mockProvider.On("GetWithHeaders", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)

	config := OauthConfigs{
		Provider:        mockProvider,
		RefreshInterval: time.Minute,
	}

	keyProvider := NewOAuth(config)

	issuedAtPast := time.Now().Add(-10 * time.Minute)
	issuedAtFuture := time.Now().Add(10 * time.Minute)

	opts := []ClaimOption{
		WithTrustedIssuer("test-issuer"),
		WithValidAudiences("test-audience"),
		EnforceExpiryCheck(),
		IssuedAfter(issuedAtPast),
		IssuedBefore(issuedAtFuture),
	}

	middlewareFn := OAuth(keyProvider, opts...)
	handler := middlewareFn(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	}))

	t.Run("Valid token", func(t *testing.T) {
		token := createValidJWT("test-kid", "test-issuer", "test-audience",
			issuedAtPast.Add(1*time.Minute), privateKey)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Expired token", func(t *testing.T) {
		token := createValidJWT("test-kid", "test-issuer", "test-audience",
			time.Now().Add(-20*time.Minute), privateKey)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func createValidJWT(kid, issuer, audience string, issuedAt time.Time, key *rsa.PrivateKey) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": issuer,
		"aud": audience,
		"iat": issuedAt.Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})
	token.Header["kid"] = kid

	signedToken, _ := token.SignedString(key)
	return signedToken
}

func createJWKSResponse(publicKey rsa.PublicKey) string {
	// Convert public key to JWKS format
	n := base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes())

	jwk := map[string]string{
		"kid": "test-kid",
		"kty": "RSA",
		"n":   n,
		"e":   string(e),
	}

	jwks := map[string][]map[string]string{
		"keys": {jwk},
	}

	jsonData, _ := json.Marshal(jwks)
	return string(jsonData)
}
