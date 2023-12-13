package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

const ldapAddress = "ldapstage.gofr.dev:636"

const ldapUser = "ZOPORDERSVC"

//nolint:gosec // To be replaced using environment variable
const ldapPass = "dsfh72hs6Ahsna"

//nolint:gosec // To be replaced using environment variable
const ldapToken = "Basic Wk9QT1JERVJTVkM6ZHNmaDcyaHM2QWhzbmE="

func Test_getUsernameAndPassword(t *testing.T) {
	type args struct {
		authHeader string
	}

	tests := []struct {
		name     string
		args     args
		wantUser string
		wantPass string
		wantErr  bool
	}{
		{"success", args{authHeader: "Basic dXNlcjpwYXNz"}, "user", "pass", false},
		{"invalid token", args{authHeader: "Basic a"}, "", "", true},
		{"failure", args{authHeader: "fail"}, "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUser, gotPass, gotErr := getUsernameAndPassword(tt.args.authHeader)

			if (gotErr != nil) != tt.wantErr {
				t.Errorf("getUsernameAndPassword() error = %v, wantErr %v", gotErr, tt.wantErr)
				return
			}

			if gotUser != tt.wantUser {
				t.Errorf("getUsernameAndPassword() got = %v, want %v", gotUser, tt.wantUser)
			}

			if gotPass != tt.wantPass {
				t.Errorf("getUsernameAndPassword() got1 = %v, want %v", gotPass, tt.wantPass)
			}
		})
	}
}

func Test_getGroupsFromEntries(t *testing.T) {
	type args struct {
		entries []*ldap.Entry
	}

	tests := []struct {
		name string
		args args
		want map[string]bool
	}{
		{name: "success1", args: args{entries: []*ldap.Entry{{
			Attributes: []*ldap.EntryAttribute{
				{
					Name:   "groupmembership",
					Values: []string{"cn=abc,ou=People,o=gofr.dev", "cn=xyz,ou=People,o=gofr.dev"},
				},
			},
		},
		}}, want: map[string]bool{
			"abc": true,
			"xyz": true,
		}},

		{name: "failure", args: args{entries: []*ldap.Entry{{
			Attributes: []*ldap.EntryAttribute{
				{
					Name:   "",
					Values: []string{"cn=abc,ou=People,o=gofr.dev", "cn=xyz,ou=People,o=gofr.dev"},
				},
			},
		}}}, want: map[string]bool{}},

		{name: "success2", args: args{entries: []*ldap.Entry{{
			Attributes: []*ldap.EntryAttribute{
				{
					Name:   "groupmembership",
					Values: []string{"cn=abc", ",ou=,o=,"},
				},
			},
		}}}, want: map[string]bool{"abc": true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getGroupsFromEntries(tt.args.entries); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getGroupsFromEntries() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getFromCache(t *testing.T) {
	tcs := []struct {
		key      string
		expected *CacheEntry
	}{
		{"abc", &CacheEntry{authorized: true}},
		{"xyz", nil},
	}
	l := NewLDAP(log.NewLogger(), &LDAPOptions{
		CacheInvalidationFrequency: 10,
	})
	l.cache.cache = map[string]*CacheEntry{
		"abc": {authorized: true},
	}

	for i, tc := range tcs {
		got := l.getFromCache(tc.key)
		assert.Equal(t, tc.expected, got, i)
	}
}

func Test_getEntry(t *testing.T) {
	user := "abc"
	pass := "fake password"
	tcs := []struct {
		cached   bool
		expected *CacheEntry
	}{
		{false, &CacheEntry{authorized: false}},
		{true, &CacheEntry{authorized: true}},
	}

	for i, tc := range tcs {
		l := NewLDAP(log.NewLogger(), &LDAPOptions{CacheInvalidationFrequency: 10})
		key := getCacheKey(user, pass)

		if tc.cached {
			l.cache.cache[key] = tc.expected
		}

		got := l.getEntry(user, pass)

		assert.Equal(t, tc.expected, got, "Test case failed %v", i)
	}
}

func Test_getEntryFromLDAPServer(t *testing.T) {
	t.Skip("skipping testing in short mode")

	expected := CacheEntry{authorized: true}

	l := NewLDAP(log.NewLogger(), &LDAPOptions{
		Addr:                       ldapAddress,
		CacheInvalidationFrequency: 1,
		InsecureSkipVerify:         true,
	})
	result := l.getEntryFromLDAPServer(ldapUser, ldapPass)

	assert.Equal(t, expected.authorized, result.authorized, "Test Failed \n")
}

func Test_getEntryFromLDAPServer_Cache(t *testing.T) {
	expected := CacheEntry{authorized: false}

	l := NewLDAP(log.NewLogger(), &LDAPOptions{
		Addr:                       ldapAddress,
		CacheInvalidationFrequency: 1,
		InsecureSkipVerify:         true,
	})
	result := l.getEntryFromLDAPServer("abc", "fake pass")

	assert.Equal(t, expected.authorized, result.authorized, "Test Failed \n")
}

func Test_addToCache(t *testing.T) {
	tcs := []struct {
		user      string
		pass      string
		frequency int
		expected  *CacheEntry
	}{
		{"abc", "fake pass", 10, &CacheEntry{authorized: true}},
		{"xyz", "fake pass", 0, nil},
	}
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	for _, tc := range tcs {
		options := LDAPOptions{
			CacheInvalidationFrequency: tc.frequency,
		}
		l := NewLDAP(logger, &options)
		l.addToCache(tc.user, map[string]bool{})

		entry := l.getFromCache(tc.user)
		if (entry == nil && tc.expected != nil) || (entry != nil && tc.expected == nil) {
			t.Errorf("Expected: %v, Got: %v", tc.expected, entry)
		}
	}
}

func Test_callLDAP(t *testing.T) {
	t.Skip("skipping testing in short mode")

	tcs := []struct {
		user         string
		pass         string
		errorMessage string
	}{
		{"abc", "fake pass", "Invalid Credentials"},
		{ldapUser, ldapPass, ""},
	}
	l := NewLDAP(log.NewLogger(), &LDAPOptions{
		Addr:                       ldapAddress,
		CacheInvalidationFrequency: 1,
		InsecureSkipVerify:         true,
	})

	_, _ = l.callLdap("", "")
	initialCount := runtime.NumGoroutine()

	for i, tc := range tcs {
		// Need to have a check for the return value as well
		_, err := l.callLdap(tc.user, tc.pass)

		if err == nil && tc.errorMessage != "" {
			t.Errorf("Expecting error %v received nil", tc.errorMessage)
		} else if err != nil && !strings.Contains(err.Error(), tc.errorMessage) {
			t.Errorf("Expecting error %v but got %v", tc.errorMessage, err.Error())
		}

		// Need to ensure that the go rountine count is not increasing with every call to callLdap method
		currentCount := runtime.NumGoroutine()
		assert.Equal(t, initialCount, currentCount, i)
	}
}

func Test_invalidateCache(t *testing.T) {
	key := "test123"
	l := NewLDAP(log.NewLogger(), &LDAPOptions{
		CacheInvalidationFrequency: 1,
	})

	l.addToCache(key, map[string]bool{})

	time.Sleep(1 * time.Second)

	got := l.getFromCache(key)

	if got != nil {
		t.Errorf("FAILED, Expected: nil, Got: %v", got)
	}
}

func Test_methodInCSV(t *testing.T) {
	testCases := []struct {
		method    string
		methodCSV string
		want      bool
	}{
		{http.MethodGet, " GET,POST", true},
		{http.MethodPost, "GET,POST", true},
		{"POS", "GET,POST", false},
	}

	for i, tc := range testCases {
		got := methodInCSV(tc.method, tc.methodCSV)
		assert.Equal(t, tc.want, got, i)
	}
}

type MockHandlerForLDAP struct{}

// ServeHTTP is used for testing different panic recovery cases
func (r *MockHandlerForLDAP) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Response == nil {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`"if you see this, the request was served"`))

		return
	}

	w.WriteHeader(req.Response.StatusCode)
	resp, err := io.ReadAll(req.Response.Body)

	if err == nil {
		_, _ = w.Write(resp)
	}
}

func TestLDAP_ValidToken(t *testing.T) {
	t.Skip("skipping testing in short mode")

	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	handler := LDAP(logger, &LDAPOptions{
		RegexToMethodGroup: map[string][]MethodGroup{
			"hello": {{
				Method: "GET,POST",
				Group:  "order-service",
			}},
		},
		Addr:                       ldapAddress,
		CacheInvalidationFrequency: 10,
		InsecureSkipVerify:         true,
	})(&MockHandlerForLDAP{})
	req := httptest.NewRequest("GET", "/hello", http.NoBody)

	if len(ldapToken) > 0 {
		req.Header.Set("Authorization", ldapToken)
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	errResp := errors.MultipleErrors{}
	_ = json.Unmarshal(w.Body.Bytes(), &errResp)
	// Check if error is being logged.
	if errResp.Errors != nil && !strings.Contains(b.String(), "Authorization") {
		t.Errorf("Failed testcase %v Middleware Error is not logged", "valid token")
	}
}
func TestLDAP_InvalidToken(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	handler := LDAP(logger, &LDAPOptions{
		RegexToMethodGroup: map[string][]MethodGroup{
			"hello": {{
				Method: "GET,POST",
				Group:  "order-service",
			}},
		},

		Addr:                       ldapAddress,
		CacheInvalidationFrequency: 10,
		InsecureSkipVerify:         true,
	})(&MockHandlerForLDAP{})

	testcases := []struct {
		name         string
		target       string
		expectedCode int
	}{
		{"missing headers", "/hello", 401},
		{"exempt path", "/.well-known/heartbeat", 200},
	}

	for _, tc := range testcases {
		req := httptest.NewRequest("GET", tc.target, http.NoBody)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, tc.expectedCode, w.Code)

		errResp := errors.MultipleErrors{}
		_ = json.Unmarshal(w.Body.Bytes(), &errResp)
		// Check if error is being logged.
		if errResp.Errors != nil && !strings.Contains(b.String(), "Authorization") {
			t.Errorf("Failed testcase %v Middleware Error is not logged", tc.name)
		}
	}
}
func TestLDAP_MissingOptions(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	handler := LDAP(logger, &LDAPOptions{})(&MockHandlerForLDAP{})
	req := httptest.NewRequest("GET", "/.well-known/heartbeat", http.NoBody)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	errResp := errors.MultipleErrors{}
	_ = json.Unmarshal(w.Body.Bytes(), &errResp)
	// Check if error is being logged.
	if errResp.Errors != nil && !strings.Contains(b.String(), "Authorization") {
		t.Errorf("Test case failed, Middleware Error is not logged")
	}
}
func TestLdap_ValidatePassword(t *testing.T) {
	t.Skip("skipping testing in short mode")

	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	l := NewLDAP(logger, &LDAPOptions{
		RegexToMethodGroup: map[string][]MethodGroup{
			"/sample": {
				{"GET, POST", "order-service"},
				{"PUT, POST", ""},
			},
		},
		Addr:                       ldapAddress,
		CacheInvalidationFrequency: 10,
		InsecureSkipVerify:         true,
	})

	testCases := []struct {
		token string
		err   error
	}{
		{ldapToken, nil},
		{"Basic Wk9QT1JERVJTVkM6ZHNmaDcyaHM2QWhzZA==", ErrUnauthenticated},
	}

	for _, testCase := range testCases {
		req := httptest.NewRequest("GET", "/sample", http.NoBody)
		req.Header.Set("Authorization", testCase.token)

		err := l.Validate(logger, req)
		assert.Equal(t, testCase.err, err)
	}
}

func TestLdap_Validate(t *testing.T) {
	t.Skip("skipping testing in short mode")

	testCases := []struct {
		regex  string
		group1 string
		group2 string
		method string
		url    string
		token  string
		err    error
	}{
		{"/sample", "grp1, order-service", "", http.MethodGet, "/sample/abc", ldapToken, nil},
		{"/sample", "order-service", "", http.MethodGet, "/sample/abc", ldapToken, nil},
		{"/sample", "", "order-service", http.MethodPut, "/sample/abc", ldapToken, nil},
		{"/sample", "invalid-group", "", http.MethodGet, "/sample/abc", ldapToken, ErrUnauthorised},
		{"/sample", "invalid-group", "", http.MethodGet, "/sample/abc", "Basic UHJhdGltOjEyMzQ=", ErrUnauthenticated},
	}
	for i, testCase := range testCases {
		b := new(bytes.Buffer)
		logger := log.NewMockLogger(b)
		l := NewLDAP(logger, &LDAPOptions{
			RegexToMethodGroup: map[string][]MethodGroup{
				testCase.regex: {
					{"GET, POST", testCase.group1},
					{"PUT, POST", testCase.group2},
				},
			},
			Addr:                       ldapAddress,
			CacheInvalidationFrequency: 10,
			InsecureSkipVerify:         true,
		})
		req := httptest.NewRequest(testCase.method, testCase.url, http.NoBody)

		if len(testCase.token) > 0 {
			req.Header.Set("Authorization", testCase.token)
		}

		err := l.Validate(logger, req)
		assert.Equal(t, testCase.err, err, "Test Failed %v \n", i)
	}
}

func TestLdap_Validate_InvalidToken_Error(t *testing.T) {
	testCases := []struct {
		regex  string
		group1 string
		group2 string
		method string
		url    string
		token  string
		err    error
	}{
		{`api(`, "", "order-service", http.MethodGet, "/sample/api", "", nil},
		{"/sample", "", "order-service", http.MethodGet, "/sample/abc", "", nil},
		{"/sample", "order-service", "", http.MethodPut, "/sample/abc", "", nil},
		{"/sample", "", "order-service", http.MethodPost, "/sample/abc", "", nil},
		{"/sample", "invalid-group", "", http.MethodGet, "/sample/abc", "Basic U1Z", ErrInvalidToken},
	}
	for i, testCase := range testCases {
		b := new(bytes.Buffer)
		logger := log.NewMockLogger(b)
		l := NewLDAP(logger, &LDAPOptions{
			RegexToMethodGroup: map[string][]MethodGroup{
				testCase.regex: {
					{"GET, POST", testCase.group1},
					{"PUT, POST", testCase.group2},
				},
			},
			Addr:                       ldapAddress,
			CacheInvalidationFrequency: 10,
			InsecureSkipVerify:         true,
		})
		req := httptest.NewRequest(testCase.method, testCase.url, http.NoBody)

		if testCase.token != "" {
			req.Header.Set("Authorization", testCase.token)
		}

		err := l.Validate(logger, req)
		assert.Equal(t, testCase.err, err, "Test Failed %v \n", i)
	}
}

func TestLDAP_GetRequiredGroups(t *testing.T) {
	tests := []struct {
		methodGroups []MethodGroup
		method       string
		groups       []string
	}{
		{[]MethodGroup{{Method: "GET, POST", Group: "order-service"}}, "GET",
			[]string{"order-service"}},
		{[]MethodGroup{{Method: "GET, POST", Group: "order-service-read, order-service-write"}}, "POST",
			[]string{"order-service-read", "order-service-write"}},
		{[]MethodGroup{{Method: "GET, POST", Group: "order-service-read, order-service-write"},
			{Method: "PUT, POST", Group: "order-service-read, order-service-write "}}, "GET",
			[]string{"order-service-read", "order-service-write"}},
		{[]MethodGroup{{Method: "GET, POST", Group: ""}}, "GET", nil},
		{[]MethodGroup{{"GET", "group1"}, {"GET", "group2"}}, "GET", []string{"group1"}},
	}

	for i, tc := range tests {
		gotGroups := getRequiredGroups(tc.methodGroups, tc.method)
		assert.Equal(t, tc.groups, gotGroups, i)
	}
}

func TestLDAP_ValidateGroups(t *testing.T) {
	testCases := []struct {
		groups []string
		entry  CacheEntry
		result bool
	}{
		{[]string{"group1", "group2"}, CacheEntry{groups: map[string]bool{"group1": true}}, true},
		{[]string{"group1", "group2"}, CacheEntry{groups: map[string]bool{"group1": false}}, false},
		{[]string{"group1", "group2"}, CacheEntry{groups: map[string]bool{"group1": false, "group2": true}}, true},
		{[]string{"group1"}, CacheEntry{groups: map[string]bool{}}, false},
		{[]string{""}, CacheEntry{groups: map[string]bool{"group1": true}}, true},
		{[]string{}, CacheEntry{groups: map[string]bool{}}, true},
	}
	for i, tc := range testCases {
		got := validateGroups(tc.groups, tc.entry)
		assert.Equal(t, tc.result, got, i)
	}
}

func Test_getListFromCSV(t *testing.T) {
	testCases := []struct {
		input  string
		output []string
	}{
		{"group1", []string{"group1"}},
		{"group1, group2", []string{"group1", "group2"}},
		{"group1, group2 ", []string{"group1", "group2"}},
		{" group1,group2", []string{"group1", "group2"}},
		{"", nil},
		{"  ", nil},
	}
	for i, tc := range testCases {
		got := getListFromCSV(tc.input)
		assert.Equal(t, tc.output, got, i)
	}
}
