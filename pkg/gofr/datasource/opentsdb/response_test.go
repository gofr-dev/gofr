package opentsdb

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errReadBody = errors.New("read body failed")

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errReadBody }

func TestSendRequest_Errors(t *testing.T) {
	tests := []struct {
		desc       string
		ctx        context.Context
		parsedResp response
		mockCall   func(m *MockhttpClient)
		expErrMsg  string
	}{
		{
			desc:       "request creation fails with nil context",
			ctx:        nil,
			parsedResp: &AggregatorsResponse{},
			mockCall:   func(*MockhttpClient) {},
			expErrMsg:  "failed to create request",
		},
		{
			desc:       "response body read fails",
			ctx:        t.Context(),
			parsedResp: &AggregatorsResponse{},
			mockCall: func(m *MockhttpClient) {
				m.EXPECT().Do(gomock.Any()).Return(&http.Response{
					StatusCode: http.StatusOK, Body: io.NopCloser(errReader{}),
				}, nil)
			},
			expErrMsg: "failed to read response body",
		},
		{
			desc:       "default unmarshal fails",
			ctx:        t.Context(),
			parsedResp: &PutResponse{},
			mockCall: func(m *MockhttpClient) {
				m.EXPECT().Do(gomock.Any()).Return(&http.Response{
					StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("not-json")),
				}, nil)
			},
			expErrMsg: "failed to unmarshal response body",
		},
		{
			desc:       "custom parser fails",
			ctx:        t.Context(),
			parsedResp: &AggregatorsResponse{},
			mockCall: func(m *MockhttpClient) {
				m.EXPECT().Do(gomock.Any()).Return(&http.Response{
					StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("not-json")),
				}, nil)
			},
			expErrMsg: "failed to parse response body through custom parser",
		},
		{
			desc:       "client error status code",
			ctx:        t.Context(),
			parsedResp: &AggregatorsResponse{},
			mockCall: func(m *MockhttpClient) {
				m.EXPECT().Do(gomock.Any()).Return(&http.Response{
					StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`[]`)),
				}, nil)
			},
			expErrMsg: "status code: 404",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client, mockHTTP := setOpenTSDBTest(t)
			tc.mockCall(mockHTTP)

			err := client.sendRequest(tc.ctx, http.MethodGet, "http://localhost:4242/api", "", tc.parsedResp)

			require.ErrorContains(t, err, tc.expErrMsg)
		})
	}
}

func TestIsValidOperateMethod(t *testing.T) {
	tests := []struct {
		desc   string
		method string
		exp    bool
	}{
		{desc: "empty method", method: "  ", exp: false},
		{desc: "lowercase post", method: "post", exp: true},
		{desc: "put", method: http.MethodPut, exp: true},
		{desc: "delete", method: http.MethodDelete, exp: true},
		{desc: "get is not allowed", method: http.MethodGet, exp: false},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.exp, (&Client{}).isValidOperateMethod(tc.method))
		})
	}
}

func TestOperateAnnotation_Errors(t *testing.T) {
	anno := &Annotation{StartTime: 1, TSUID: "0001"}

	tests := []struct {
		desc       string
		annotation any
		resp       any
		method     string
		mockCall   func(m *MockhttpClient)
		expErr     error
	}{
		{
			desc:       "invalid annotation type",
			annotation: Annotation{},
			resp:       &AnnotationResponse{},
			method:     http.MethodPost,
			mockCall:   func(*MockhttpClient) {},
			expErr:     errInvalidArgument,
		},
		{
			desc:       "invalid response type",
			annotation: anno,
			resp:       &QueryResponse{},
			method:     http.MethodPost,
			mockCall:   func(*MockhttpClient) {},
			expErr:     errInvalidResponseType,
		},
		{
			desc:       "invalid operate method",
			annotation: anno,
			resp:       &AnnotationResponse{},
			method:     http.MethodGet,
			mockCall:   func(*MockhttpClient) {},
			expErr:     errUnexpected,
		},
		{
			desc:       "http request failure",
			annotation: anno,
			resp:       &AnnotationResponse{},
			method:     http.MethodPost,
			mockCall: func(m *MockhttpClient) {
				m.EXPECT().Do(gomock.Any()).Return(nil, errRequestFailed)
			},
			expErr: errRequestFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client, mockHTTP := setOpenTSDBTest(t)
			tc.mockCall(mockHTTP)

			err := client.operateAnnotation(t.Context(), tc.annotation, tc.resp, tc.method, "Annotation")

			require.ErrorIs(t, err, tc.expErr)
		})
	}
}

func TestGetCustomParser_Responses(t *testing.T) {
	tests := []struct {
		desc    string
		resp    response
		input   string
		expErr  bool
		expResp response
	}{
		{
			desc: "query response from object", resp: &QueryResponse{},
			input:   `{"queryRespCnts":[{"metric":"cpu"}]}`,
			expResp: &QueryResponse{QueryRespCnts: []QueryRespItem{{Metric: "cpu"}}},
		},
		{
			desc: "query last response from array", resp: &QueryLastResponse{},
			input:   `[{"metric":"cpu"}]`,
			expResp: &QueryLastResponse{QueryRespCnts: []QueryRespLastItem{{Metric: "cpu"}}},
		},
		{desc: "version invalid json", resp: &VersionResponse{}, input: `bad`, expErr: true, expResp: &VersionResponse{}},
		{desc: "aggregators invalid json", resp: &AggregatorsResponse{}, input: `bad`, expErr: true, expResp: &AggregatorsResponse{}},
		{desc: "annotation empty body", resp: &AnnotationResponse{}, input: ``, expResp: &AnnotationResponse{}},
		{desc: "annotation invalid json", resp: &AnnotationResponse{}, input: `bad`, expErr: true, expResp: &AnnotationResponse{}},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client, _ := setOpenTSDBTest(t)

			err := tc.resp.getCustomParser(client.logger)([]byte(tc.input))

			assert.Equal(t, tc.expErr, err != nil)
			assert.Equal(t, tc.expResp, tc.resp)
		})
	}
}

func TestPutResponse_GetCustomParser(t *testing.T) {
	assert.Nil(t, (&PutResponse{}).getCustomParser(nil))
}
