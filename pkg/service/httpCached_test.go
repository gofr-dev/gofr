package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/log"
	"gofr.dev/pkg/middleware"
)

func TestCacheGet(t *testing.T) {
	type resp struct {
		FirstName string `json:"FirstName"`
	}

	// test server for testing cached response
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		re := resp{FirstName: "Hello"}
		reBytes, _ := json.Marshal(re)
		w.Header().Set("Content-type", "application/json")
		_, _ = w.Write(reBytes)
	}))

	// initialisation
	b := new(bytes.Buffer)
	cacher := NewHTTPServiceWithOptions(ts.URL, log.NewMockLogger(b), &Options{Cache: &Cache{mockCache{}, 0, nil}})

	// expected responses
	r := resp{FirstName: "Hello"}
	expected, _ := json.Marshal(r)
	expectedLog := "getting cached response"

	_, _ = cacher.Get(context.TODO(), "brand", map[string]interface{}{"page": 1, "name": "xyz"})
	// getting cached response even when order of parameter is different
	body, _ := cacher.Get(context.TODO(), "brand", map[string]interface{}{"name": "xyz", "page": 1})

	if !reflect.DeepEqual(body.Body, expected) {
		t.Errorf("Failed.Expected %v\tGot %v", expected, body)
	}

	if !strings.Contains(b.String(), expectedLog) {
		t.Errorf("Failed.Expected Log %v\tGot %v", expectedLog, b.String())
	}

	ts.Close()
}

type mockCache struct{}

func (m mockCache) Get(key string) ([]byte, error) {
	if strings.Contains(key, "error") || strings.Contains(key, "GET") {
		return nil, nil
	}

	if strings.HasSuffix(key, "_status") {
		return []byte(`200`), nil
	}

	return []byte(`{"FirstName":"Hello"}`), nil
}

func (m mockCache) Delete(string) error {
	return nil
}

func (m mockCache) Set(key string, _ []byte, _ time.Duration) error {
	if strings.Contains(key, "error") {
		return errors.New("could not connect to redis")
	}

	return nil
}

func TestHTTPCacheSetError(t *testing.T) {
	type resp struct {
		FirstName string `json:"FirstName"`
	}

	// test server for testing
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		re := resp{FirstName: "Hello"}
		reBytes, _ := json.Marshal(re)
		w.Header().Set("Content-type", "application/json")
		_, _ = w.Write(reBytes)
	}))

	// initialisation
	b := new(bytes.Buffer)
	cacher := NewHTTPServiceWithOptions(ts.URL, log.NewMockLogger(b), &Options{Cache: &Cache{mockCache{}, 0, nil}})

	expectedLog := "unable to cache, err:could not connect to redis"

	_, _ = cacher.Get(context.TODO(), "error", nil)

	if !strings.Contains(b.String(), expectedLog) {
		t.Errorf("Failed.Expected Log %v\tGot %v", expectedLog, b.String())
	}

	ts.Close()
}

func TestHTTPCacheError(t *testing.T) {
	expectedErr := url.Error{
		Op:  "Get",
		URL: "/GET",
	}
	cacher := NewHTTPServiceWithOptions("", log.NewLogger(), &Options{Cache: &Cache{mockCache{}, 0, nil}})

	_, err := cacher.Get(context.TODO(), "GET", nil)
	v, ok := err.(*url.Error)

	if !ok || (v.URL != expectedErr.URL || v.Op != expectedErr.Op) {
		t.Errorf("Failed.Expected %v\tGot %v", expectedErr, err)
	}
}

func TestCacheGetWithHeaders(t *testing.T) {
	type resp struct {
		FirstName string `json:"FirstName"`
	}

	// test server for testing cached response
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("id") == "1" && r.Header.Get("entity") == "test" {
			re := resp{FirstName: "Hello"}
			reBytes, _ := json.Marshal(re)
			w.Header().Set("Content-type", "application/json")
			_, _ = w.Write(reBytes)
		}
	}))

	// initialisation
	b := new(bytes.Buffer)
	_ = config.NewGoDotEnvProvider(log.NewLogger(), "../../configs")

	cacher := NewHTTPServiceWithOptions(ts.URL, log.NewMockLogger(b),
		&Options{Headers: map[string]string{"id": "1"}, Cache: &Cache{mockCache{}, 0, nil}})

	// expected responses
	r := resp{FirstName: "Hello"}
	expected, _ := json.Marshal(r)
	expectedLog := "getting cached response"

	_, _ = cacher.GetWithHeaders(context.TODO(), "brand", map[string]interface{}{"page": 1, "name": "xyz"},
		map[string]string{"entity": "test"})
	// getting cached response even when order of parameter is different
	body, _ := cacher.GetWithHeaders(context.TODO(), "brand", map[string]interface{}{"name": "xyz", "page": 1},
		map[string]string{"entity": "test"})

	if !reflect.DeepEqual(body.Body, expected) {
		t.Errorf("Failed.Expected %v\tGot %v", expected, body)
	}

	if !strings.Contains(b.String(), expectedLog) {
		t.Errorf("Failed.Expected Log %v\tGot %v", expectedLog, b.String())
	}

	ts.Close()
}

func TestGetHeaders(t *testing.T) {
	testCases := []struct {
		existingHeaders map[string]string
		auth            string
		// output
		headers map[string]string
	}{
		{nil, "", map[string]string{"X-Correlation-ID": "123"}},
		{map[string]string{"a": "abc"}, "", map[string]string{"a": "abc", "X-CorrelationId": "123"}},
		{nil, "123",
			map[string]string{"X-CorrelationId": "123", "Authorization": "123"}},
	}

	for i := range testCases {
		ctx := context.TODO()
		ctx = context.WithValue(ctx, middleware.CorrelationIDKey, "123")

		c := cachedHTTPService{httpService: &httpService{auth: testCases[i].auth}}
		headers := c.getHeaders(ctx, testCases[i].existingHeaders)

		if len(headers) != len(testCases[i].headers) {
			t.Errorf("[TESTCASE%d]headers were not set", i+1)
		}
	}
}

func Test_GetHeaders_All(t *testing.T) {
	expectedHeaders := map[string]string{"X-Correlation-ID": "123", "True-Client-IP": "127.0.0.1",
		"X-Zopsmart-Channel": "api", "X-Authenticated-UserId": "990", "X-Zopsmart-Tenant": "zopsmart", "X-B3-TraceID": "123"}

	ctx := context.TODO()
	ctx = context.WithValue(ctx, middleware.CorrelationIDKey, "123")
	ctx = context.WithValue(ctx, middleware.ClientIPKey, "127.0.0.1")
	ctx = context.WithValue(ctx, middleware.ZopsmartChannelKey, "api")
	ctx = context.WithValue(ctx, middleware.AuthenticatedUserIDKey, "990")
	ctx = context.WithValue(ctx, middleware.ZopsmartTenantKey, "zopsmart")
	ctx = context.WithValue(ctx, middleware.B3TraceIDKey, "123")

	headers := cachedHTTPService{httpService: &httpService{}}.getHeaders(ctx, nil)

	if len(headers) != len(expectedHeaders) {
		t.Errorf("headers are not set")
	}
}

func TestGenerateKey(t *testing.T) {
	api := "customer"
	params := map[string]interface{}{"id": 123}
	headers := map[string]string{"X-Zopsmart-Tenant": "zopsmart"}

	b, _ := json.Marshal(params)

	expectedKey := api + ":" + string(b)

	h := sha256.New()
	b, _ = json.Marshal(headers)
	_, _ = h.Write(b)

	expectedKey += ":" + fmt.Sprintf("%x", h.Sum(nil))

	key := generateKey(api, params, headers)

	if key != expectedKey {
		t.Errorf("generation of key failed.\ngot %v\nexpected %v", key, expectedKey)
	}
}

// check the condition when user passed the keyGeneratorFunc
func TestCacheGetWithHeadersPassedKey(t *testing.T) {
	type resp struct {
		FirstName string `json:"FirstName"`
	}

	// test server for testing cached response
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		re := resp{FirstName: "Hello"}
		reBytes, _ := json.Marshal(re)
		w.Header().Set("Content-type", "application/json")
		_, _ = w.Write(reBytes)
	}))

	// initialisation

	httpCache := &Cache{
		Cacher: mockCache{},
		TTL:    120,
		KeyGenerator: KeyGenerator(func(url string, params map[string]interface{}, headers map[string]string) string {
			// passing the random string
			return "TnJkYXRhIJoiSGVsbG8gIn0K"
		}),
	}

	cacher := NewHTTPServiceWithOptions(ts.URL, log.NewMockLogger(io.Discard), &Options{
		Cache: httpCache,
	})

	r := resp{FirstName: "Hello"}
	expected, _ := json.Marshal(r)

	_, _ = cacher.GetWithHeaders(context.TODO(), "brand", nil, nil)
	body, _ := cacher.GetWithHeaders(context.TODO(), "brand", nil, nil)

	if !reflect.DeepEqual(body.Body, expected) {
		t.Errorf("Failed.Expected Log %v\tGot %v", expected, body)
	}
}
