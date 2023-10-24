package handlers

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gofr.dev/examples/using-awssns/entity"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/notifier"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func initializeTests(t *testing.T, method string, body io.Reader) (*MockNotifier, *gofr.Context) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockService := NewMockNotifier(mockCtrl)
	app := gofr.New()
	app.Notifier = mockService
	req := httptest.NewRequest(method, "/dummy", body)
	r := request.NewHTTPRequest(req)
	c := gofr.NewContext(nil, r, app)

	return mockService, c
}

func TestPublisherHandler(t *testing.T) {
	tests := []struct {
		desc    string
		wantErr error
		body    []byte
	}{
		{desc: "Success Case", body: []byte(`{"name": "GOFR", "message":  "hi"}`)},
		{desc: "Failure Case", wantErr: errors.EntityNotFound{}},
	}

	for _, tc := range tests {
		mockService, ctx := initializeTests(t, http.MethodPost, bytes.NewBuffer(tc.body))
		attr := map[string]interface{}{
			"email":   "test@abc.com",
			"version": 1.1,
			"key":     []interface{}{1, 1.999, "value"},
		}

		var message *entity.Message

		_ = ctx.Bind(&message)

		mockService.EXPECT().Publish(message, attr).Return(tc.wantErr)

		_, err := Publisher(ctx)

		assert.ErrorIsf(t, err, tc.wantErr, "%v Error expected %v but got : %v", tc.desc, tc.wantErr, err)
	}
}

func TestSubscriberHandler(t *testing.T) {
	mockService, ctx := initializeTests(t, http.MethodGet, nil)

	tests := []struct {
		desc    string
		wantErr error
	}{
		{desc: "Success Case"},
		{desc: "Failure Case", wantErr: errors.EntityNotFound{}},
	}

	for _, tc := range tests {
		data := map[string]interface{}{}

		mockService.EXPECT().SubscribeWithResponse(&data).Return(&notifier.Message{}, tc.wantErr)

		_, err := Subscriber(ctx)

		assert.ErrorIsf(t, err, tc.wantErr, "%v Error expected %v but got : %v", tc.desc, tc.wantErr, err)
	}
}
