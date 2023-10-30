package user

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/using-http-service/models"
	"gofr.dev/examples/using-http-service/services"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/log"
	svc "gofr.dev/pkg/service"
)

func initializeTest(t *testing.T) (service, mockHTTPService, *gofr.Context) {
	ctrl := gomock.NewController(t)
	mock := services.NewMockHTTPService(ctrl)
	mockSvc := mockHTTPService{MockHTTPService: mock}
	s := New(mockSvc)

	g := gofr.Gofr{Logger: log.NewMockLogger(io.Discard)}
	ctx := gofr.NewContext(nil, nil, &g)

	return s, mockSvc, ctx
}

func TestService_Get(t *testing.T) {
	s, mockSvc, ctx := initializeTest(t)
	serviceErr := errors.Error("error from service call")

	serverErr := errors.MultipleErrors{StatusCode: http.StatusInternalServerError, Errors: []error{&errors.Response{
		Code:     "Internal Server Error",
		Reason:   "connection timed out",
		DateTime: errors.DateTime{Value: "2021-11-03T11:01:13.124Z", TimeZone: "IST"},
	}}}

	errResp := &svc.Response{
		StatusCode: http.StatusInternalServerError,
		Body: []byte(`
		{
			"errors": [
				{
					"code": "Internal Server Error",
					"reason": "connection timed out",
					"datetime": {
						"value": "2021-11-03T11:01:13.124Z",
						"timezone": "IST"
					}
				}
			]
		}`),
	}

	resp := &svc.Response{
		StatusCode: http.StatusOK,
		Body: []byte(`
		{
    		"data": {
        		"name": "Vikash",
        		"company": "gofr.dev"
    		}
		}`),
	}

	tests := []struct {
		desc     string
		mockResp *svc.Response // mock response from get call
		mockErr  error         // mock error from get call
		output   models.User
		err      error
	}{
		{"error from get call", nil, serviceErr, models.User{}, serviceErr},
		{"error response from get call", errResp, nil, models.User{}, serverErr},
		{"success case", resp, nil, models.User{Name: "Vikash", Company: "gofr.dev"}, nil},
	}

	for i, tc := range tests {
		mockSvc.EXPECT().Get(ctx, "user/Vikash", nil).Return(tc.mockResp, tc.mockErr)

		output, err := s.Get(ctx, "Vikash")

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i+1, tc.desc)

		assert.Equal(t, tc.output, output, "TEST[%d], failed.\n%s", i+1, tc.desc)
	}
}

func TestService_GetBindError(t *testing.T) {
	s, mockSvc, ctx := initializeTest(t)

	var u models.User

	data := struct {
		Data interface{} `json:"data"`
	}{Data: &u}

	resp := struct {
		Errors []errors.Response `json:"errors"`
	}{}

	var bindErr = &errors.Response{
		Code:   "Bind Error",
		Reason: "failed to bind response",
		Detail: errors.Error("bind error"),
	}

	tests := []struct {
		desc     string
		respType interface{}
		mockResp *svc.Response // mock response from get call
	}{
		{"error in binding error response", &resp, &svc.Response{StatusCode: http.StatusBadRequest, Body: []byte(`invalid body`)}},
		{"error in binding data response", &data, &svc.Response{StatusCode: http.StatusOK, Body: []byte(`invalid body`)}},
	}

	for i, tc := range tests {
		mockSvc.EXPECT().Get(ctx, "user/Vikash", nil).Return(tc.mockResp, nil)
		mockSvc.EXPECT().Bind(tc.mockResp.Body, tc.respType).Return(errors.Error("bind error"))

		output, err := s.Get(ctx, "Vikash")

		assert.Equal(t, bindErr, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, models.User{}, output, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func Test_GetErrResponse(t *testing.T) {
	s, _, _ := initializeTest(t)
	expResp := errors.MultipleErrors{
		StatusCode: http.StatusBadRequest,
		Errors: []error{
			&errors.Response{
				Code:     "Request error",
				Reason:   "Invalid Request",
				Detail:   "Invalid parameter",
				DateTime: errors.DateTime{Value: "2021-11-03T11:01:13.124Z", TimeZone: "IST"},
			},
		},
	}

	inputbody := []byte(`
		{
			"errors": [
				{
					"code": "Request error",
					"reason": "Invalid Request",
					"detail": "Invalid parameter",
					"datetime": {
						"value": "2021-11-03T11:01:13.124Z",
						"timezone": "IST"
					}
				}
			]
		}`)

	err := s.getErrorResponse(inputbody, http.StatusBadRequest)
	assert.Equal(t, expResp, err, "Test failed")
}

func Test_GetErrResponseBindErr(t *testing.T) {
	s, mockSvc, _ := initializeTest(t)

	bindError := &errors.Response{
		Code:   "Bind Error",
		Reason: "failed to bind response",
		Detail: errors.Error("bind error"),
	}

	resp := struct {
		Errors []errors.Response `json:"errors"`
	}{}

	inputBody := []byte(`invalid body`)

	mockSvc.EXPECT().Bind(inputBody, &resp).Return(errors.Error("bind error"))

	err := s.getErrorResponse(inputBody, http.StatusBadRequest)
	assert.Equal(t, bindError, err, "Test failed.")
}

func Test_Bind(t *testing.T) {
	s, mockSvc, _ := initializeTest(t)

	bindErr := &errors.Response{
		Code:   "Bind Error",
		Reason: "failed to bind response",
		Detail: errors.Error("bind error"),
	}
	testcases := []struct {
		desc    string
		body    []byte
		expErr  error
		mockErr error
	}{
		{"Bind error case", []byte("Hello world"), bindErr, errors.Error("bind error")},
		{"Success case", []byte("Hello world"), nil, nil},
	}

	for i, tc := range testcases {
		var resp string

		mockSvc.EXPECT().Bind(tc.body, &resp).Return(tc.mockErr)
		err := s.bind(tc.body, &resp)
		assert.Equal(t, tc.expErr, err, "Test [%d] failed.\n%s", i, tc.desc)
	}
}

type mockHTTPService struct {
	*services.MockHTTPService // embed mock of HTTPService interface
}

// override the Bind method
func (m mockHTTPService) Bind(resp []byte, i interface{}) error {
	if err := json.Unmarshal(resp, i); err != nil {
		return m.MockHTTPService.Bind(resp, i)
	}

	return nil
}
