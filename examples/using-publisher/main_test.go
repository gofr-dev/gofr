package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gofr.dev/pkg/gofr/testutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestExamplePublisherError(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	host := fmt.Sprint("http://localhost:", configs.HTTPPort)

	go main()
	time.Sleep(200 * time.Millisecond)

	testCases := []struct {
		desc               string
		path               string
		body               []byte
		expectedStatusCode int
	}{
		{"valid order", "/publish-order", []byte(`{"data":{"orderId":"123","status":"pending"}}`), http.StatusInternalServerError},
		{"valid product", "/publish-product", []byte(`{"data":{"productId":"123","price":"599"}}`), http.StatusInternalServerError},
	}

	client := http.Client{}

	for i, tc := range testCases {
		req, _ := http.NewRequest(http.MethodPost, host+tc.path, bytes.NewBuffer(tc.body))
		req.Header.Set("content-type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err, "TEST[%d] %s failed", i, tc.desc)
		defer resp.Body.Close()

		assert.Equal(t, tc.expectedStatusCode, resp.StatusCode, "TEST[%d] %s failed", i, tc.desc)
	}
}

func TestOrderFunction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name           string
		body           string
		expectError    bool
		publishError   error
		expectedResult interface{}
	}{
		{"valid order", `{"orderId":"123","status":"pending"}`, false, nil, "Published"},
		{"invalid JSON", `{"orderId":,"status":"pending"}`, true, nil, nil},
		{"publish error", `{"orderId":"123","status":"pending"}`, true, fmt.Errorf("publish failed"), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockContainer, mocks := container.NewMockContainer(t)

			switch {
			case tt.publishError != nil:
				mocks.PubSub.EXPECT().Publish(gomock.Any(), "order-logs", gomock.Any()).Return(tt.publishError)
			case !tt.expectError:
				mocks.PubSub.EXPECT().Publish(gomock.Any(), "order-logs", gomock.Any()).Return(nil)
			}

			testHandler(t, tt.name, order, mockContainer, tt.body, tt.expectError, tt.expectedResult)
		})
	}
}

func TestProductFunction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name           string
		body           string
		expectError    bool
		publishError   error
		expectedResult interface{}
	}{
		{"valid product", `{"productId":"123","price":"599"}`, false, nil, "Published"},
		{"invalid JSON", `{"productId":,"price":"599"}`, true, nil, nil},
		{"publish error", `{"productId":"123","price":"599"}`, true, fmt.Errorf("publish failed"), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockContainer, mocks := container.NewMockContainer(t)

			switch {
			case tt.publishError != nil:
				mocks.PubSub.EXPECT().Publish(gomock.Any(), "products", gomock.Any()).Return(tt.publishError)
			case !tt.expectError:
				mocks.PubSub.EXPECT().Publish(gomock.Any(), "products", gomock.Any()).Return(nil)
			}

			testHandler(t, tt.name, product, mockContainer, tt.body, tt.expectError, tt.expectedResult)
		})
	}
}

func testHandler(t *testing.T, name string, handler func(*gofr.Context) (interface{}, error),
	container *container.Container, body string, expectError bool, expectedResult interface{}) {

	t.Run(name, func(t *testing.T) {
		ctx := &gofr.Context{
			Context:       context.Background(),
			Request:       &testRequest{Request: httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(body))), body: body},
			Container:     container,
			ContextLogger: *logging.NewContextLogger(context.Background(), container.Logger),
		}

		result, err := handler(ctx)

		assert.Equal(t, expectError, err != nil, "error presence mismatch")
		assert.Equal(t, expectedResult, result, "result mismatch")
	})
}

type testRequest struct {
	*http.Request
	body string
}

func (r *testRequest) Bind(v interface{}) error {
	return json.Unmarshal([]byte(r.body), v)
}

func (r *testRequest) Param(key string) string     { return r.URL.Query().Get(key) }
func (r *testRequest) PathParam(key string) string { return "" }
func (r *testRequest) HostName() string            { return r.Host }
func (r *testRequest) Params(key string) []string  { return r.URL.Query()[key] }
