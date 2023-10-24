package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/log"
)

func TestNewHTTPServiceWithOauthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Wrong JSON Format",}`, 400)
	}))

	tests := []struct {
		clientID     string
		clientSecret string
		url          string
		err          string
	}{
		{"", "", "http://some-random-url", ""},
		{"", "", server.URL, ""},
		{"Alice", "password", "http://some-random-url", "some-random-url"},
		{"Alice", "password", server.URL, "invalid character '}'"},
	}

	for _, tc := range tests {
		b := new(bytes.Buffer)
		logger := log.NewMockLogger(b)
		oauthOption := OAuthOption{
			ClientID:       tc.clientID,
			ClientSecret:   tc.clientSecret,
			KeyProviderURL: tc.url,
			MaxSleep:       120,
		}

		_ = NewHTTPServiceWithOptions("http://dummy-url", logger, &Options{Auth: &Auth{OAuthOption: &oauthOption}})

		time.Sleep(time.Duration(4) * time.Second)

		if !strings.Contains(b.String(), tc.err) {
			t.Errorf("Logline %v \n Does not contain: %v", b.String(), tc.err)
		}
	}
}

func Test_getNewAccessToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			res        map[string]interface{}
			statusCode int
		)

		user, _, _ := r.BasicAuth()
		if user == "user" {
			res = map[string]interface{}{
				"access_token": "dummy-token",
				"expires_in":   100,
			}

			statusCode = http.StatusOK
		} else {
			res = map[string]interface{}{
				"reason": "invalid_credentials",
			}

			statusCode = http.StatusUnauthorized
		}

		resBytes, _ := json.Marshal(res)

		w.WriteHeader(statusCode)
		_, _ = w.Write(resBytes)
	}))

	defer ts.Close()

	type args struct {
		basicAuth string
		option    *OAuthOption
	}

	tests := []struct {
		name            string
		args            args
		wantBearerToken string
		wantExp         int
	}{
		{
			"token received", args{"user:pass", &OAuthOption{KeyProviderURL: ts.URL}},
			"Bearer dummy-token", 100,
		},
		{
			"invalid credentials", args{"pass:user", &OAuthOption{KeyProviderURL: ts.URL}},
			"", 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tt.args.basicAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(tt.args.basicAuth))

			gotBearerToken, gotExp, _ := getNewAccessToken(log.NewLogger(), tt.args.basicAuth, tt.args.option)
			if gotBearerToken != tt.wantBearerToken {
				t.Errorf("getNewAccessToken() gotBearerToken = %v, want %v", gotBearerToken, tt.wantBearerToken)
			}

			if gotExp != tt.wantExp {
				t.Errorf("getNewAccessToken() gotExp = %v, want %v", gotExp, tt.wantExp)
			}
		})
	}
}

func Test_getPayload(t *testing.T) {
	const (
		audience = "https://zopsmart.com"
		scope    = "some-scope"
	)

	options := &OAuthOption{Audience: audience, Scope: scope}

	data := getPayload(options)

	tests := []struct {
		desc     string
		key      string
		expected string
	}{
		{"compares audience value", "audience", audience},
		{"compares scope value", "scope", scope},
		{"compares grant_type value", "grant_type", "client_credentials"},
	}

	for i, tc := range tests {
		assert.Equal(t, tc.expected, data[tc.key][0], "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func Test_SuccessStatusRange(t *testing.T) {
	testcases := []struct {
		input          int
		expectedOutput bool
	}{
		{100, false},
		{200, true},
		{299, true},
		{211, true},
		{300, false},
	}

	for i := range testcases {
		output := successStatusRange(testcases[i].input)
		if output != testcases[i].expectedOutput {
			t.Errorf("[TESTCASE%d]Failed. Expected: %v\tGot: %v", i+1, testcases[i].expectedOutput, output)
		}
	}
}

func Test_Log_FetchToken(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	o := OAuthOption{ClientID: "clientid", ClientSecret: "clientsecret",
		KeyProviderURL: "http://localhost:9090", MaxSleep: 1}
	h := &httpService{logger: logger, url: "http://localhost:9080"}
	expLog := "Failed to fetch token for service http://localhost:9080. Error from http://localhost:9090"

	h.setClientOauthHeader(&o)

	if !strings.Contains(b.String(), expLog) {
		t.Errorf("[FAILED] expected: %v, got: %v", expLog, b.String())
	}
}
