package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	gofrError "gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
	"gofr.dev/pkg/middleware"
)

const dummyURL = "http://dummy"

func TestService_Request(t *testing.T) {
	ts := testServer()
	defer ts.Close()

	logger := log.NewLogger()

	tests := []struct {
		name string
		url  string
		// input
		api    string
		params map[string]interface{}
		body   []byte
		// output
		wantErr bool
	}{
		{"invalid request", "http:dummyURL", "dummy", nil, []byte(`"hello"`), true},
		{"params in request", ts.URL, "dummy", map[string]interface{}{"name": "gofr"}, nil, false},
		{"invalid api", "^%", "%%", map[string]interface{}{"name": "gofr"}, nil, true},
	}

	for _, tt := range tests {
		ps := NewHTTPServiceWithOptions(tt.url, logger, nil)

		ps.SetSurgeProtectorOptions(false, "", 5)

		_, err := ps.Post(context.Background(), tt.api, tt.params, tt.body)
		if (err != nil) != tt.wantErr {
			t.Errorf("Test %v:\t error = %v, wantErr %v", tt.name, err, tt.wantErr)
			return
		}

		_, err = ps.Put(context.Background(), tt.api, tt.params, tt.body)
		if (err != nil) != tt.wantErr {
			t.Errorf("Test %v:\t error = %v, wantErr %v", tt.name, err, tt.wantErr)
			return
		}

		_, err = ps.Patch(context.Background(), tt.api, tt.params, tt.body)
		if (err != nil) != tt.wantErr {
			t.Errorf("Test %v:\t error = %v, wantErr %v", tt.name, err, tt.wantErr)
			return
		}

		_, err = ps.Delete(context.Background(), tt.api, tt.body)
		if (err != nil) != tt.wantErr {
			t.Errorf("Test %v:\t error = %v, wantErr %v", tt.name, err, tt.wantErr)
			return
		}
	}
}

func TestService_Get(t *testing.T) {
	ts := testServer()
	defer ts.Close()

	tests := []struct {
		name string
		URL  string
		// input
		api    string
		params map[string]interface{}

		wantErr bool
	}{
		{"success response", ts.URL + "/", "dummy", nil, false},
		{"success response", ts.URL + "/", "dummy", map[string]interface{}{"name": "gofr", "location": []string{"CA, NYC"}}, false},
		{"error invalid api", "^%", "&^", nil, true},
	}

	for _, tt := range tests {
		ps := NewHTTPServiceWithOptions(tt.URL, log.NewMockLogger(io.Discard), nil)

		ps.SetConnectionPool(1, 1*time.Second)
		ps.SetSurgeProtectorOptions(false, "", 5)

		_, err := ps.Get(context.Background(), tt.api, tt.params)
		if (err != nil) != tt.wantErr {
			t.Errorf("Test %v:\t  error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestService_CustomRetry(t *testing.T) {
	ts := retryTestServer()
	defer ts.Close()

	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	svc := NewHTTPServiceWithOptions(ts.URL, logger, &Options{NumOfRetries: 1})

	svc.CustomRetry = func(logger log.Logger, err error, statusCode, attemptCount int) bool {
		if statusCode == http.StatusBadRequest {
			logger.Logf("got error %v on attempt %v", err, attemptCount)
			return true
		}

		return false
	}

	resp, err := svc.Get(context.Background(), "dummy", nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("got unexpected error %v", err)
	}
}

// The TestService_GetRetry function is testing the http service retry mechanism
func TestService_GetRetry(t *testing.T) {
	tests := []struct {
		url          string
		numOfRetries int
		timeout      time.Duration
		wantErr      bool
	}{
		// no retry on success
		{"server-url", 10, 0, false},
		// no retry on error when retry is not configured
		{"/sample-service", 0, 0, true},
		// no retry on non-timeout error
		{"/index", 2, 0, true},
		// retry 5 times on timeout error and fail
		{"server-url", 5, 10, true},
	}

	for i, tc := range tests {
		timeout := tc.timeout

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			re := map[string]interface{}{"name": "gofr"}
			reBytes, _ := json.Marshal(re)
			// By adding sleep we are trying to increase the duration of response, so when we set ResponseHeaderTimeout value
			// less than the sleep value, we will be able to get timeout
			time.Sleep((timeout * 2) * time.Millisecond)
			w.Header().Set("Content-type", "application/json")
			_, _ = w.Write(reBytes)
		}))

		opts := &Options{NumOfRetries: tc.numOfRetries}
		// Assigning the test server url
		if tc.url == "server-url" {
			tc.url = ts.URL
		}

		h := NewHTTPServiceWithOptions(tc.url, log.NewMockLogger(io.Discard), opts)
		// The non-zero value of ResponseHeaderTimeout, specifies the amount of time to wait for a server's response headers
		// after fully writing the request
		http.DefaultTransport.(*http.Transport).ResponseHeaderTimeout = timeout * time.Millisecond

		_, err := h.Get(context.Background(), "dummy", nil)
		if (err != nil) != tc.wantErr {
			t.Errorf("Test[%v] FAILED. error = %v, wantErr %v", i+1, err, tc.wantErr)
		}

		ts.Close()
	}
}

// The TestService_CustomPostRetryBodyError function is testing the http service retry mechanism for repeated requests having a body
func TestService_CustomRetryRequestBodyError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`"data":"hello"`))
	}))

	// creating a new service call with test-server url , mock logger and number of retries
	h := NewHTTPServiceWithOptions(ts.URL, log.NewMockLogger(io.Discard), &Options{NumOfRetries: 3})
	h.CustomRetry = func(logger log.Logger, err error, statusCode, attemptCount int) bool {
		if statusCode == http.StatusCreated {
			return false
		}

		if err != nil && attemptCount < 2 {
			logger.Errorf("Retrying because of err: ", err)
			return true
		}

		return true
	}

	_, err := h.Post(context.Background(), "/dummy", nil, []byte(`{"name":"gofr"}`))

	ts.Close()

	// error should occur if io.NopCloser is not used for req.Body inside custom-Retry func. Error: "http: ContentLength=15 with Body length 0"
	assert.NoError(t, err)
}

func TestServiceLog_String(t *testing.T) {
	l := &callLog{
		Method:  http.MethodGet,
		URI:     "test",
		Headers: map[string]string{"Content-Type": "application/json"},
	}

	expected := `{"correlationId":"","type":"","timestamp":0,"duration":0,"method":"GET","uri":"test",` +
		`"responseCode":0,"headers":{"Content-Type":"application/json"}}`
	if got := l.String(); got != expected {
		t.Errorf("String() = %v, want %v", got, expected)
	}
}

func Test_Client_ctx_cancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		re := `<name>Gofr</name>`
		reBytes, _ := xml.Marshal(re)
		_, _ = w.Write(reBytes)
	}))

	defer ts.Close()

	ctx := context.Background()
	ps := NewHTTPServiceWithOptions(ts.URL, log.NewMockLogger(io.Discard), nil)
	//nolint:govet // ignoring the cancel function
	ctx, _ = context.WithTimeout(ctx, 2*time.Nanosecond)

	_, err := ps.Get(ctx, "dummy", nil)
	if err == nil {
		t.Errorf("Get() = Request canceled error expected")
	}
}

func initializeOauthTestServer(addSleep bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if addSleep {
			// used for token blocking
			time.Sleep(time.Millisecond * 40)
		}
		sampleTokenResponse := map[string]interface{}{
			"expires_in":   10,
			"access_token": "sample_token",
			"token_type":   "bearer",
		}

		_ = json.NewEncoder(w).Encode(sampleTokenResponse)
	}))
}

func Test_OAuth_ctx_cancel_RequestTimeout(t *testing.T) {
	testServer := initializeOauthTestServer(false)
	defer testServer.Close()

	logger := log.NewMockLogger(io.Discard)
	testcases := []struct {
		ctxTimeout time.Duration
		reqTimeout time.Duration
		err        error
	}{
		{time.Nanosecond * 5, time.Second * 10, RequestCanceled{}},
		{time.Second * 10, time.Nanosecond * 5, gofrError.Timeout{URL: testServer.URL}},
	}

	for i := range testcases {
		oauth := OAuthOption{
			ClientID:        "client_id",
			ClientSecret:    "clientSecret",
			KeyProviderURL:  testServer.URL,
			Scope:           "test:data",
			WaitForTokenGen: true,
		}

		svc := NewHTTPServiceWithOptions(testServer.URL, logger, &Options{Auth: &Auth{OAuthOption: &oauth}})
		svc.Timeout = testcases[i].reqTimeout

		//nolint:govet // ignoring the cancel function
		ctx, _ := context.WithTimeout(context.TODO(), testcases[i].ctxTimeout)

		_, err := svc.Get(ctx, "dummy", nil)
		if !reflect.DeepEqual(err, testcases[i].err) {
			t.Errorf("[TESTCASE%d]Failed.\nExpected %v\nGot %v\n", i+1, testcases[i].err, err)
		}
	}
}

func TestCallError(t *testing.T) {
	octr := otelhttp.NewTransport(nil)
	c := &http.Client{Transport: octr}
	client := &httpService{
		url:    "sample service",
		sp:     surgeProtector{isEnabled: true, retryFrequencySeconds: 500},
		logger: log.NewLogger(),
		Client: c,
	}
	expectedErr := ErrServiceDown{URL: "sample service"}

	_, err := client.call(context.TODO(), http.MethodGet, "", nil, nil, nil)

	if !reflect.DeepEqual(err, expectedErr) {
		t.Errorf("Failed.Expected %v\tGot %v", expectedErr, err)
	}
}

func TestLogError(t *testing.T) {
	b := new(bytes.Buffer)
	octr := otelhttp.NewTransport(nil)

	c := &http.Client{Transport: octr}
	client := &httpService{
		url:       "sample service",
		sp:        surgeProtector{isEnabled: true, retryFrequencySeconds: 500},
		logger:    log.NewMockLogger(b),
		Client:    c,
		isHealthy: true,
	}
	ctx := context.WithValue(context.TODO(), middleware.AuthorizationHeader, "some auth")

	_, _ = client.call(ctx, http.MethodGet, "", nil, nil, nil)
	expected := "unsupported protocol scheme"

	if !strings.Contains(b.String(), expected) {
		t.Errorf("FAILED expected %v,got: %v", expected, b.String())
	}

	if !strings.Contains(b.String(), "ERROR") {
		t.Errorf("Failed. Expected type to be ERROR")
	}
}

func Test_SetContentType(t *testing.T) {
	testcases := []struct {
		body        []byte
		contentType string
	}{
		{[]byte("hello"), "text/plain"},
		{[]byte(`<resp>hello</resp>`), "application/xml"},
		{[]byte(`{"name":"garry"}`), "application/json"},
		{nil, ""},
	}

	for i, v := range testcases {
		req := httptest.NewRequest(http.MethodGet, "/dummy", nil)
		setContentTypeAndAcceptHeader(req, v.body)

		contentType := req.Header.Get("content-type")

		if contentType != v.contentType {
			t.Errorf("[TESCASE %d]Failed. Got %v\tExpected %v\n", i+1, contentType, v.contentType)
		}
	}
}

func Test_SetAcceptHeader(t *testing.T) {
	expectedHeader := "application/json,application/xml,text/plain"
	req := httptest.NewRequest(http.MethodGet, "/dummy", nil)
	setContentTypeAndAcceptHeader(req, nil)

	header := req.Header.Get("accept")

	if header != expectedHeader {
		t.Errorf("Failed. Got %v\tExpected %v\n", header, expectedHeader)
	}
}

func Test_SetSurgeProtectorOptions(t *testing.T) {
	var h httpService

	isEnabled := true
	customHeartbeatURL := "/.fake-heartbeat"
	retryFrequencySeconds := 1

	h.sp.logger = log.NewLogger()

	h.SetSurgeProtectorOptions(isEnabled, customHeartbeatURL, retryFrequencySeconds)

	if h.sp.isEnabled != isEnabled || h.sp.customHeartbeatURL != customHeartbeatURL ||
		h.sp.retryFrequencySeconds != retryFrequencySeconds || h.isHealthy {
		t.Errorf("FAILED, Expected: %v, %v, %v, Got: %v, %v, %v, %v", isEnabled, customHeartbeatURL,
			retryFrequencySeconds, h.sp.isEnabled, h.sp.customHeartbeatURL, h.sp.retryFrequencySeconds, h.isHealthy)
	}
}

func TestCorrelationIDPropagation(t *testing.T) {
	correlationIDs := []string{"1YUHS767SHD", "", "OUDIDd78f78d"}
	for i := range correlationIDs {
		h := httpService{}

		ctx := context.WithValue(context.Background(), middleware.CorrelationIDKey, correlationIDs[i])
		req, _ := h.createReq(ctx, http.MethodGet, "/", nil, nil, nil)

		correlationID := req.Header.Get("X-Correlation-ID")
		if correlationID != correlationIDs[i] {
			t.Errorf("Failed.Expected %v\tGot %v", correlationIDs[i], correlationID)
		}
	}
}

func TestService_CorrelationIDLog(t *testing.T) {
	ts := testServer()
	defer ts.Close()

	correlationID := "81ADIDDNODID"
	buf := new(bytes.Buffer)

	ps := NewHTTPServiceWithOptions(ts.URL, log.NewMockLogger(buf), nil)
	ctx := context.WithValue(context.Background(), middleware.CorrelationIDKey, correlationID)
	_, _ = ps.Get(ctx, "", nil)

	if !strings.Contains(buf.String(), correlationID) {
		t.Errorf("could not log the right correlationID")
	}
}

func TestHttpService_SetHeaders(t *testing.T) {
	httpSvc := NewHTTPServiceWithOptions(dummyURL, log.NewLogger(), nil)

	ctx := context.WithValue(context.TODO(), middleware.ClientIPKey, "123.234.545.894")
	ctx = context.WithValue(ctx, middleware.AuthenticatedUserIDKey, "2")
	ctx = context.WithValue(ctx, middleware.B3TraceIDKey, "3434")

	req, _ := httpSvc.createReq(ctx, http.MethodGet, "", nil, nil, nil)
	if req.Header.Get("X-Authenticated-UserId") != "2" || req.Header.Get("X-B3-TraceID") != "3434" {
		t.Error("setting of headers failed")
	}
}

func TestHttpService_SetAuthClientIP(t *testing.T) {
	s := NewHTTPServiceWithOptions(dummyURL, log.NewLogger(),
		&Options{Headers: map[string]string{"Authorization": "se31-2fhhvhjf-9049"}})
	ctx := context.WithValue(context.TODO(), middleware.ClientIPKey, "123.234.545.894")
	req, _ := s.createReq(ctx, http.MethodGet, "", nil, nil, nil)

	if req.Header.Get("True-Client-Ip") != "123.234.545.894" || req.Header.Get("Authorization") != "se31-2fhhvhjf-9049" {
		t.Errorf("setting of auth failed")
	}
}

func TestHttpService_PropagateHeaders(t *testing.T) {
	httpSvc := httpService{
		Client: &http.Client{},
		url:    dummyURL,
	}

	httpSvc.PropagateHeaders("X-Custom-Header")

	//nolint:revive,staticcheck // cannot make the key a constant
	ctx := context.WithValue(context.TODO(), "X-Custom-Header", "ab")

	req, _ := httpSvc.createReq(ctx, http.MethodGet, "", nil, nil, nil)
	if req.Header.Get("X-Custom-Header") != "ab" {
		t.Error("setting of custom headers failed")
	}
}

// TestCallLog tests if Authorization is not logged and AppData is logged
func TestCallLog(t *testing.T) {
	ts := testServer()
	defer ts.Close()

	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	h := NewHTTPServiceWithOptions(ts.URL, logger, nil)

	h.logger.AddData("key", "value")

	data := &sync.Map{}
	data.Store("key", "value")

	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.LogDataKey("appLogData"), data)

	_, _ = h.Get(ctx, "", nil)

	if !strings.Contains(b.String(), `"data":{"key":"value"}`) {
		t.Errorf("logline doesn't contain appData")
	}
}

func TestErrorLogString(t *testing.T) {
	l := &errorLog{
		Method:  http.MethodGet,
		URI:     "test",
		Headers: map[string]string{"Content-Type": "application/json"},
		Message: "cannot connect",
	}

	expected := `{"correlationId":"","type":"","timestamp":0,"duration":0,"method":"GET","uri":"test",` +
		`"headers":{"Content-Type":"application/json"},"message":"cannot connect"}`
	if got := l.String(); got != expected {
		t.Errorf("String() = %v, want %v", got, expected)
	}
}

func TestCustomHeaderPropagation(t *testing.T) {
	type expectedFields struct {
		id            string
		entity        string
		correlationID string
	}

	testCases := []struct {
		name                string
		serviceLevelHeaders map[string]string
		methodLevelHeaders  map[string]string
		want                expectedFields
	}{
		{"no headers passed", nil, nil, expectedFields{}},
		{"service level headers passed", map[string]string{"X-Correlation-ID": "123456789"}, nil, expectedFields{correlationID: "123456789"}},
		{"both service level and request level headers passed", map[string]string{"X-Correlation-ID": "123456789"},
			map[string]string{"id": "1234", "entity": "test"}, expectedFields{correlationID: "123456789", id: "1234", entity: "test"}},
		{"only request level headers passed", nil, map[string]string{
			"id": "1234", "entity": "test"}, expectedFields{id: "1234", entity: "test"}},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			h := httpService{customHeaders: tt.serviceLevelHeaders}

			req, _ := h.createReq(context.TODO(), http.MethodGet, "/", nil, nil, tt.methodLevelHeaders)

			id := req.Header.Get("id")
			entity := req.Header.Get("entity")
			correlationID := req.Header.Get("X-Correlation-ID")

			if id != tt.want.id || entity != tt.want.entity || correlationID != tt.want.correlationID {
				t.Errorf("Failed, Got: id=%v, entity=%v, correlationId=%v \nWant: id=%v, entity=%v, correlationId=%v",
					id, entity, correlationID, tt.want.id, tt.want.entity, tt.want.correlationID)
			}
		})
	}
}

func TestHeaderPriority(t *testing.T) {
	testCases := []struct {
		name                string
		serviceLevelHeaders map[string]string
		methodLevelHeaders  map[string]string
		wantContentType     string
		wantClientIP        string
	}{
		{"no headers passed", nil, nil, "application/json", "0.0.0.0"},
		{"service level headers passed", map[string]string{"Content-Type": "image/png", "True-Client-Ip": "1.1.1.1"}, nil,
			"image/png", "1.1.1.1"},
		{"both service level and request level headers passed", map[string]string{"Content-Type": "image/png", "True-Client-Ip": "1.1.1.1"},
			map[string]string{"Content-Type": "image/jpeg", "True-Client-Ip": "2.2.2.2"}, "image/jpeg", "2.2.2.2"},
		{"only request level headers passed", nil, map[string]string{"Content-Type": "image/png", "True-Client-Ip": "2.2.2.2"},
			"image/png", "2.2.2.2"},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			h := httpService{customHeaders: tt.serviceLevelHeaders}
			h.PropagateHeaders("Content-Type")

			ctx := context.WithValue(context.TODO(), middleware.ClientIPKey, "0.0.0.0")
			body, _ := json.Marshal(map[string]interface{}{"entity": "test"})
			req, _ := h.createReq(ctx, http.MethodPost, "/", nil, body, tt.methodLevelHeaders)

			contentType := req.Header.Get("Content-Type")
			if contentType != tt.wantContentType {
				t.Errorf("Failed, Got: contentType=%v \nWant: id=%v", contentType, tt.wantContentType)
			}

			clientIP := req.Header.Get("True-Client-Ip")
			if clientIP != tt.wantClientIP {
				t.Errorf("Failed, Got: clientIP=%v \nWant: id=%v", clientIP, tt.wantClientIP)
			}
		})
	}
}

func TestNewHTTPAuthService(t *testing.T) {
	user := "Alice"
	pass := "12345"
	url := dummyURL
	svc := NewHTTPServiceWithOptions(url, log.NewLogger(), &Options{Auth: &Auth{UserName: user, Password: pass}})
	authStr := "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))

	exp := &httpService{
		url:  url,
		auth: authStr,
	}
	if exp.auth != svc.auth {
		t.Errorf("Expected: %v \nGot: %v", exp, svc)
	}
}

// TestHTTPCookieLogging checks, Cookie is getting logged or not for http client.
func TestHTTPCookieLogging(t *testing.T) {
	b := new(bytes.Buffer)
	url := dummyURL
	h := NewHTTPServiceWithOptions(url, log.NewMockLogger(b), nil)
	_, _ = h.call(context.TODO(), http.MethodGet, "", nil, nil, map[string]string{"Cookie": "Some-Random-Value"})

	x := b.String()
	if strings.Contains(x, `"Cookie":"Some-Random-Value"`) {
		t.Errorf("Error: Expected no cookie, Got: %v", x)
	}
}

func Test_AuthCall(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	url := dummyURL

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sampleTokenResponse := map[string]interface{}{
			"expires_in":   10,
			"access_token": "sample_token",
			"token_type":   "bearer",
		}
		_ = json.NewEncoder(w).Encode(sampleTokenResponse)
	}))

	tests := []struct {
		option      *Options
		auth        string
		expectedErr error
	}{
		{nil, "", nil},
		{&Options{Auth: &Auth{UserName: "user", Password: "secret"}}, "Basic dXNlcjpzZWNyZXQ=", nil},
		{&Options{Auth: &Auth{OAuthOption: &OAuthOption{ClientID: "Alice", ClientSecret: "alice_secret", KeyProviderURL: server.URL,
			Scope: "some_scope"}}}, "Bearer sample_token", nil},
		{&Options{Auth: &Auth{UserName: "user", Password: "abc", OAuthOption: &OAuthOption{}}}, "", ErrToken},
		{&Options{Auth: &Auth{OAuthOption: &OAuthOption{}}}, "", ErrToken},
		{&Options{Auth: &Auth{OAuthOption: &OAuthOption{ClientID: "Bob", ClientSecret: "bob_secret", KeyProviderURL: url,
			Scope: "some_scope"}}}, "", ErrToken},
	}

	for i, tc := range tests {
		var err error

		httpSvc := NewHTTPServiceWithOptions(url, logger, tc.option)

		if tc.expectedErr != nil {
			_, err = httpSvc.call(context.Background(), "", "", nil, nil, nil)

			assert.Contains(t, b.String(), tc.expectedErr, i)
		}

		time.Sleep(time.Duration(2) * time.Second)
		assert.Equal(t, tc.expectedErr, err, i)
		assert.Equal(t, tc.auth, httpSvc.auth, i)
	}
}

func TestGetUsername(t *testing.T) {
	type args struct {
		authHeader string
	}

	tests := []struct {
		name     string
		args     args
		wantUser string
		wantErr  error
	}{
		{"success", args{authHeader: "Basic dXNlcjpwYXNz"}, "user", nil},
		{"invalid token", args{authHeader: "Basic a"}, "", middleware.ErrInvalidToken},
		{"failure", args{authHeader: "fail"}, "", middleware.ErrInvalidHeader},
		{name: "error missing", args: args{authHeader: ""}, wantUser: "", wantErr: middleware.ErrMissingHeader},
	}

	for i, tc := range tests {
		gotUser, gotErr := getUsername(tc.args.authHeader)

		if gotErr != tc.wantErr {
			t.Errorf("FAILED[%v]\tgot:%v, wantErr:%v", i+1, gotErr, tc.wantErr)
			return
		}

		if gotUser != tc.wantUser {
			t.Errorf("FAILED [%v] getUsername() got: %v, want:%v", i+1, gotUser, tc.wantUser)
		}
	}
}

func Test_authorizationHeaderSet(t *testing.T) {
	testCases := []struct {
		headers             map[string]string
		authorizationHeader string
	}{
		{
			headers:             map[string]string{"Authorization": "some-random", "X-Correlation-ID": "Random"},
			authorizationHeader: "Basic Z29mcjpwd2Q=",
		},
	}

	for i, tc := range testCases {
		setAuthHeader(tc.headers, tc.authorizationHeader)

		if tc.headers["Authorization"] != "gofr" {
			t.Errorf("TestCases[%v] , expected: %v , got: %v ", i+1, "td", tc.headers["Authorization"])
		}
	}
}

// Test_ResponseHeaders tests if headers are getting attached to the downstream service response.
func Test_ResponseHeaders(t *testing.T) {
	ts := testServer()
	defer ts.Close()

	httpSvc := NewHTTPServiceWithOptions(ts.URL, log.NewLogger(), nil)
	resp, _ := httpSvc.call(context.Background(), "", "", nil, nil, nil)

	assert.Equal(t, "application/json", resp.GetHeader("Content-type"))
}

func Test_createReq(t *testing.T) {
	ts := testServer()
	defer ts.Close()
	testcase := []struct {
		desc        string
		method      string
		target      string
		params      map[string]interface{}
		expectedURL string
	}{
		{"multiple backslashes in POST", http.MethodPost, "////////post", nil, ts.URL + "/////////post"},
		{"single backslashes in GET", http.MethodGet, "get", nil, ts.URL + "/get"},
		{"single backslashes and params of type []string in GET", http.MethodGet, "get",
			map[string]interface{}{"id": []string{"1", "2", "3"}}, ts.URL + "/get?id=1&id=2&id=3"},
		{"single backslashes and params of type string in GET", http.MethodGet, "get", map[string]interface{}{"id": "1"}, ts.URL + "/get?id=1"},
		{"multiple backslashes in PUT", http.MethodPut, "//put", nil, ts.URL + "///put"},
		{"single backslashes in PATCH", http.MethodPatch, "patch", nil, ts.URL + "/patch"},
	}

	h := NewHTTPServiceWithOptions(ts.URL, log.NewLogger(), nil)
	for _, tc := range testcase {
		req, err := h.createReq(context.Background(), tc.method, tc.target, tc.params, nil, nil)
		if err != nil {
			t.Errorf("DESC: %v Error: %v", tc.desc, err)
		}

		if req.Method != tc.method || req.URL.String() != tc.expectedURL {
			t.Errorf("DESC: %v\nExpectedMethod: %v\nGotMethod: %v\nExpectedURL: %v\nGotURL: %v",
				tc.desc, tc.method, req.Method, tc.expectedURL, req.URL)
		}
	}
}

func testServer() *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		re := map[string]interface{}{"name": "gofr"}
		reBytes, _ := json.Marshal(re)
		w.Header().Set("Content-type", "application/json")
		_, _ = w.Write(reBytes)
	}))

	return ts
}

func retryTestServer() *httptest.Server {
	var cnt int

	// returns http.StatusBadRequest for first retry and http.StatusOK for second retry
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cnt == 0 {
			w.WriteHeader(http.StatusBadRequest)
			cnt++

			return
		}

		w.WriteHeader(http.StatusOK)
	}))

	return ts
}

func Test_PreCallStatusCode(t *testing.T) {
	testcases := []struct {
		h                  httpService
		expectedStatusCode int
		expectedError      error
	}{
		{httpService{auth: "test token", authOptions: authOptions{isSet: true}, isHealthy: false},
			http.StatusInternalServerError, ErrServiceDown{}},
		{httpService{auth: "test token", authOptions: authOptions{isSet: true}, isHealthy: true}, 0, nil},
		{httpService{auth: "", authOptions: authOptions{isSet: true}, isHealthy: true}, http.StatusUnauthorized, ErrToken},
	}

	for i := range testcases {
		statusCode, err := testcases[i].h.preCall()

		if reflect.DeepEqual(err, &testcases[i].expectedError) {
			t.Errorf("[TESTCASE%d]Failed. Expected: %v\tGot: %v", i+1, testcases[i].expectedError, err)
		}

		if statusCode != testcases[i].expectedStatusCode {
			t.Errorf("[TESTCASE%d]Failed. Expected: %v\tGot: %v", i+1, testcases[i].expectedStatusCode, statusCode)
		}
	}
}

func Test_PreTokenObtained(t *testing.T) {
	testServer := initializeOauthTestServer(false)
	defer testServer.Close()

	logger := log.NewMockLogger(io.Discard)

	oauth := OAuthOption{
		ClientID:        "clientID",
		ClientSecret:    "clientSecret",
		KeyProviderURL:  testServer.URL,
		Scope:           "test:data",
		WaitForTokenGen: true,
	}

	svc := NewHTTPServiceWithOptions("http://dummy", logger, &Options{Auth: &Auth{OAuthOption: &oauth},
		SurgeProtectorOption: &SurgeProtectorOption{Disable: true}})
	svc.isHealthy = true

	_, err := svc.preCall()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if svc.auth == "" {
		t.Errorf("Expected token to be set")
	}
}

// Test_logCall_disableQueryParamLogging to test behavior of logCall when skipQParamLogging is set
//
//nolint:dupl // not duplication, testing different function
func Test_logCall_disableQueryParamLogging(t *testing.T) {
	h, b, logMsg := initializeLogTest()
	l := &callLog{}

	// when skipQParamLogging is true
	h.skipQParamLogging = true
	l.Params = map[string]interface{}{"test-param": "test-value"}

	h.logCall(l, map[string]string{}, time.Time{}, "")

	assert.NotContainsf(t, b.String(), logMsg, "Test Failed: skipQParamLogging is true")

	// when skipQParamLogging is false
	h.skipQParamLogging = false
	l.Params = map[string]interface{}{"test-param": "test-value"}

	h.logCall(l, map[string]string{}, time.Time{}, "")

	assert.Containsf(t, b.String(), logMsg, "Test Failed: skipQParamLogging is false")
}

// Test_logError_disableQueryParamLogging to test behavior of logError when skipQParamLogging is set
//
//nolint:dupl // not duplication, testing different function
func Test_logError_disableQueryParamLogging(t *testing.T) {
	httpSvc, b, logMsg := initializeLogTest()
	l := &errorLog{}

	// when skipQParamLogging is true
	httpSvc.skipQParamLogging = true
	l.Params = map[string]interface{}{"test-param": "test-value"}

	httpSvc.logError(l, map[string]string{}, time.Time{}, "")

	assert.NotContainsf(t, b.String(), logMsg, "Test Failed: skipQParamLogging is true")

	// when skipQParamLogging is false
	httpSvc.skipQParamLogging = false
	l.Params = map[string]interface{}{"test-param": "test-value"}

	httpSvc.logError(l, map[string]string{}, time.Time{}, "")

	assert.Containsf(t, b.String(), logMsg, "Test Failed: skipQParamLogging is false")
}

func initializeLogTest() (*httpService, *bytes.Buffer, string) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	httpSvc := &httpService{logger: logger}
	logMsg := "\"params\":{\"test-param\":\"test-value\"}"

	return httpSvc, b, logMsg
}
