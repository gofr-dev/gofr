package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/metrics"
	"gofr.dev/pkg/gofr/request"
)

type mockStore struct{}

const redisKey = "someKey"
const size = 64

func (m mockStore) Get(_ *gofr.Context, key string) (string, error) {
	switch key {
	case redisKey:
		return "someValue", nil
	case "errorKey":
		return "", mockStore{}
	default:
		return "", mockStore{}
	}
}

func (m mockStore) Set(_ *gofr.Context, key, value string, _ time.Duration) error {
	if key == redisKey && value == "someValue" {
		return mockStore{}
	}

	return nil
}

func (m mockStore) Delete(_ *gofr.Context, key string) error {
	switch key {
	case redisKey:
		return nil
	case "errorKey":
		return mockStore{}
	default:
		return nil
	}
}

func (m mockStore) Error() string {
	return "some mocked error"
}

func TestModel_GetKey(t *testing.T) {
	m := New(mockStore{})

	app := gofr.New()

	tests := []struct {
		desc string
		key  string
		resp interface{}
		err  error
	}{
		{"get with key", redisKey, "someValue", nil},
		{"get with empty key", "", nil, errors.MissingParam{Param: []string{"key"}}},
		{"get with error key", "errorKey", nil, mockStore{}},
	}

	for i, tc := range tests {
		r := httptest.NewRequest(http.MethodGet, "http://dummy", nil)

		req := request.NewHTTPRequest(r)
		ctx := gofr.NewContext(nil, req, app)

		if tc.key != "" {
			ctx.SetPathParams(map[string]string{
				"key": tc.key,
			})
		}

		_, err := m.GetKey(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestModel_DeleteKey(t *testing.T) {
	m := New(mockStore{})

	app := gofr.New()

	tests := []struct {
		desc string
		key  string
		err  error
	}{
		{"delete success", redisKey, nil},
		{"delete with empty key", "", errors.MissingParam{Param: []string{"key"}}},
		{"delete with error key", "errorKey", deleteErr{}},
	}

	for i, tc := range tests {
		r := httptest.NewRequest(http.MethodDelete, "http://dummy", nil)

		req := request.NewHTTPRequest(r)
		ctx := gofr.NewContext(nil, req, app)

		if tc.key != "" {
			ctx.SetPathParams(map[string]string{
				"key": tc.key,
			})
		}

		_, err := m.DeleteKey(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestModel_SetKey(t *testing.T) {
	m := New(mockStore{})

	app := gofr.New()
	mockMetric := metrics.NewMockMetric(gomock.NewController(t))
	app.Metric = mockMetric

	tests := []struct {
		desc    string
		body    []byte
		err     error
		counter string
		label   string
	}{
		{"set key with invalid body", []byte(`{`), invalidBodyErr{}, InvalidBodyCounter, "failed"},
		{"set key with invalid input", []byte(`{"someKey":"someValue"}`), invalidInputErr{}, NumberOfSetsCounter, "failed"},
		{"set key success", []byte(`{"someKey123": "123"}`), nil, NumberOfSetsCounter, "succeeded"},
	}

	for i, tc := range tests {
		r := httptest.NewRequest(http.MethodPost, "http://dummy", bytes.NewReader(tc.body))

		req := request.NewHTTPRequest(r)
		ctx := gofr.NewContext(nil, req, app)

		length, _ := strconv.ParseFloat(ctx.Header("Content-Length"), size)

		mockMetric.EXPECT().SetGauge(ReqContentLengthGauge, length).Return(nil).AnyTimes()
		mockMetric.EXPECT().IncCounter(tc.counter).Return(nil).AnyTimes()
		mockMetric.EXPECT().IncCounter(NumberOfSetsCounter, tc.label).Return(nil).AnyTimes()

		_, err := m.SetKey(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestSetKey_SetGaugeError(t *testing.T) {
	app := gofr.New()
	m := New(mockStore{})

	r := httptest.NewRequest(http.MethodPost, "http://dummy", nil)

	req := request.NewHTTPRequest(r)
	ctx := gofr.NewContext(nil, req, app)
	mockMetric := metrics.NewMockMetric(gomock.NewController(t))
	ctx.Metric = mockMetric

	length, _ := strconv.ParseFloat(ctx.Header("Content-Length"), size)

	expErr := errors.Error("error case")
	mockMetric.EXPECT().SetGauge(ReqContentLengthGauge, length).Return(expErr)

	_, err := m.SetKey(ctx)
	assert.Equal(t, expErr, err)
}

func TestSetKey_InvalidBodyCounterError(t *testing.T) {
	app := gofr.New()
	m := New(mockStore{})
	r := httptest.NewRequest(http.MethodPost, "http://dummy", bytes.NewReader([]byte(`{`)))
	req := request.NewHTTPRequest(r)
	ctx := gofr.NewContext(nil, req, app)
	mockMetric := metrics.NewMockMetric(gomock.NewController(t))
	ctx.Metric = mockMetric

	length, _ := strconv.ParseFloat(ctx.Header("Content-Length"), size)

	mockMetric.EXPECT().SetGauge(ReqContentLengthGauge, length).Return(nil)

	expErr := errors.Error("error case")
	mockMetric.EXPECT().IncCounter(InvalidBodyCounter).Return(expErr)

	_, err := m.SetKey(ctx)
	assert.Equal(t, expErr, err)
}

func TestSetKey_IncCounterError(t *testing.T) {
	tcs := []struct {
		desc  string
		body  []byte
		label string
	}{
		{"invalid body", []byte(`{"`), "failed"},
		{"error key", []byte(`{"someKey":"someValue"}`), "failed"},
		{"valid key", []byte(`{"someKey1":"someValue1"}`), "succeeded"},
	}

	app := gofr.New()
	m := New(mockStore{})
	mockMetric := metrics.NewMockMetric(gomock.NewController(t))
	app.Metric = mockMetric
	expErr := errors.Error("error case")

	mockMetric.EXPECT().IncCounter(InvalidBodyCounter).Return(nil)

	for i, tc := range tcs {
		r := httptest.NewRequest(http.MethodPost, "http://dummy", bytes.NewReader(tc.body))
		req := request.NewHTTPRequest(r)
		ctx := gofr.NewContext(nil, req, app)

		length, _ := strconv.ParseFloat(ctx.Header("Content-Length"), size)

		mockMetric.EXPECT().SetGauge(ReqContentLengthGauge, length).Return(nil).AnyTimes()
		mockMetric.EXPECT().IncCounter(NumberOfSetsCounter, tc.label).Return(expErr).AnyTimes()

		_, err := m.SetKey(ctx)
		assert.Equal(t, expErr, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestDeleteErr_Error(t *testing.T) {
	var d deleteErr

	expected := "error: failed to delete"
	got := d.Error()

	if got != expected {
		t.Errorf("FAILED, expected: %v, got: %v", expected, got)
	}
}

func TestInvalidInputErr_Error(t *testing.T) {
	var i invalidInputErr

	expected := "error: invalid input"
	got := i.Error()

	if got != expected {
		t.Errorf("FAILED, expected: %v, got: %v", expected, got)
	}
}

func TestInvalidBodyErr_Error(t *testing.T) {
	var i invalidBodyErr

	expected := "error: invalid body"
	got := i.Error()

	if got != expected {
		t.Errorf("FAILED, expected: %v, got: %v", expected, got)
	}
}
