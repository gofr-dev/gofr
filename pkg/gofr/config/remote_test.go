package config

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

const appName = "test"

func Test_Get(t *testing.T) {
	r := RemoteConfig{
		remoteConfig: map[string]string{"APP_NAME": "gofr"},
		localConfig:  &MockConfig{Data: map[string]string{"APP_VERSION": "dev"}},
	}

	testCases := []struct {
		key   string
		value string
	}{
		{"APP_NAME", "gofr"},
		{"APP_VERSION", "dev"},
	}
	for i, tc := range testCases {
		value := r.Get(tc.key)
		assert.Equal(t, tc.value, value, "Test case [%d] failed.", i)
	}
}

func Test_GetOrDefault(t *testing.T) {
	r := RemoteConfig{
		remoteConfig: map[string]string{"APP_NAME": "gofr"},
		localConfig:  &MockConfig{Data: map[string]string{"APP_VERSION": "dev"}},
	}

	testCases := []struct {
		key   string
		value string
	}{
		{"APP_NAME", "gofr"},
		{"APP_VERSION", "dev"},
		{"HTTP_PORT", "Default"},
	}
	for i, tc := range testCases {
		value := r.GetOrDefault(tc.key, "Default")
		assert.Equal(t, tc.value, value, "Test case [%d] failed.", i)
	}
}

func testServer(cs string) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte
		switch cs {
		case "success":
			body = []byte(`{"data": [{"serviceName": "gofr-sample-api","config": {"APP_VERSION":"v0.17.0"},"userGroup":"fwk"}]}`)
		case "error":
			body = []byte(`{"data": []}`)
		}

		w.Header().Set("Content-type", "application/json")
		_, _ = w.Write(body)
	}))

	return ts
}

func Test_NewRemoteConfigProvider(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	appName := appName
	localConfig := &MockConfig{
		Data: map[string]string{"APP_VERSION": "v0.16.0",
			"REMOTE_NAMESPACE": "ZS_NAMESPACE",
		},
	}
	remoteURL := testServer("success").URL
	r := NewRemoteConfigProvider(localConfig, remoteURL, appName, logger)

	time.Sleep(5 * time.Millisecond)

	if r.remoteConfig["APP_VERSION"] != "v0.17.0" {
		t.Errorf("Test Failed. Expected: %v, got: %v", "v0.17.0", r.remoteConfig["APP_VERSION"])
	}
}

func Test_refreshConfigs(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	appName := appName
	localConfig := &MockConfig{
		Data: map[string]string{"APP_VERSION": "v0.16.0",
			"REMOTE_NAMESPACE": "ZS_NAMESPACE",
		},
	}
	remoteURL := testServer("success").URL
	remoteConfig := map[string]string{}
	freq := int(5 * time.Millisecond)

	r := RemoteConfig{remoteConfig: remoteConfig, localConfig: localConfig, logger: logger, appName: appName, url: remoteURL, frequency: freq}

	go r.refreshConfigs()

	time.Sleep(time.Second)

	if r.remoteConfig["APP_VERSION"] != "v0.17.0" {
		t.Errorf("Test Failed. Expected: %v, got: %v", "v0.17.0", r.remoteConfig["APP_VERSION"])
	}
}

func Test_refreshConfigsErr(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	appName := appName
	server := testServer("error")
	localConfig := &MockConfig{
		Data: map[string]string{"APP_VERSION": "v0.16.0",
			"REMOTE_NAMESPACE": "ZS_NAMESPACE",
		},
	}

	remoteConfig := map[string]string{}
	freq := int(5 * time.Millisecond)

	reqErr := RemoteConfig{remoteConfig: remoteConfig, localConfig: localConfig, logger: logger,
		appName: appName, url: "http://dummy", frequency: freq}

	bindErr := RemoteConfig{remoteConfig: remoteConfig, localConfig: localConfig, logger: logger,
		appName: appName, url: server.URL, frequency: freq}

	testCases := []struct {
		desc   string
		config RemoteConfig
		logMsg string
	}{
		{"request error", reqErr, `message":{"Op":"Get","URL":"http://dummy/configs?cluster`},
		{"bind error", bindErr, "Unable to find config for test"},
	}

	for i, tc := range testCases {
		go tc.config.refreshConfigs()

		time.Sleep(time.Second)

		if !strings.Contains(b.String(), tc.logMsg) {
			t.Errorf("Test Failed[%d] DESC :%v. Expected log: %v, got: %v", i, tc.desc, tc.logMsg, b.String())
		}
	}
}

func Test_getRemoteConfigs(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	validBody := []byte(`{"data": [{"serviceName": "gofr-sample-api","config": {"LOG_LEVEL": "WARN","DB_NAME":"MYSQL"},"userGroup":"fwk"}]}`)
	invalidBody := []byte(`{"data": [{"serviceName gofr-sample-api","config": {"LOG_LEVEL": "WARN","DB_NAME":"MYSQL"},"userGroup":"fwk"}]}`)

	r := &RemoteConfig{logger: logger}
	testCases := []struct {
		desc string
		body []byte
		err  error
	}{
		{"success case", validBody, nil},
		{"invalid body", invalidBody, &json.SyntaxError{}},
		{"empty body", []byte(`{"data":[]}`), errors.EntityNotFound{}},
		{"empty json response", []byte(`{}`), errors.EntityNotFound{}},
		{"invalid json response", []byte(`{"data":}`), &json.SyntaxError{}},
		{"invalid json response", []byte(`{"data":`), &json.SyntaxError{}},
	}

	for i, tc := range testCases {
		_, err := r.getRemoteConfigs(tc.body)

		assert.IsType(t, tc.err, err, "Test case [%d] failed.", i)
	}
}
