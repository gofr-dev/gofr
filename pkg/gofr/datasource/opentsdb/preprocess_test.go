package opentsdb

import (
	"math"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestParsePutErrorMsg(t *testing.T) {
	tests := []struct {
		desc      string
		resp      *PutResponse
		expErrMsg string
	}{
		{
			desc: "formats put errors",
			resp: &PutResponse{Failed: 1, Errors: []PutError{{
				Data: DataPoint{Metric: "cpu", Value: 1}, ErrorMsg: "bad metric",
			}}},
			expErrMsg: "Failed to put 1 datapoint(s) into opentsdb",
		},
		{
			desc:      "no put errors",
			resp:      &PutResponse{Failed: 2},
			expErrMsg: "Failed to put 2 datapoint(s) into opentsdb",
		},
		{
			desc: "put error cannot be marshaled",
			resp: &PutResponse{Failed: 1, Errors: []PutError{{
				Data: DataPoint{Metric: "cpu", Value: math.NaN()}, ErrorMsg: "bad",
			}}},
			expErrMsg: "unsupported value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := parsePutErrorMsg(tc.resp)

			require.ErrorContains(t, err, tc.expErrMsg)
		})
	}
}

func TestIsValidDataPoint(t *testing.T) {
	tags := map[string]string{"host": "h"}

	tests := []struct {
		desc  string
		value any
		exp   bool
	}{
		{desc: "int64 value", value: int64(1), exp: true},
		{desc: "int value", value: 1, exp: true},
		{desc: "float64 value", value: 1.5, exp: true},
		{desc: "float32 value", value: float32(1.5), exp: true},
		{desc: "string value", value: "1", exp: true},
		{desc: "unsupported value type", value: []int{1}, exp: false},
		{desc: "nil value", value: nil, exp: false},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			dp := DataPoint{Metric: "cpu", Timestamp: 1, Value: tc.value, Tags: tags}

			assert.Equal(t, tc.exp, isValidDataPoint(&dp))
		})
	}
}

func TestIsValidQueryParam(t *testing.T) {
	validQuery := SubQuery{Aggregator: "sum", Metric: "cpu"}

	tests := []struct {
		desc  string
		param *QueryParam
		exp   bool
	}{
		{desc: "no queries", param: &QueryParam{Start: 1}, exp: false},
		{desc: "invalid start", param: &QueryParam{Start: 0, Queries: []SubQuery{validQuery}}, exp: false},
		{desc: "missing aggregator", param: &QueryParam{Start: 1, Queries: []SubQuery{{Metric: "cpu"}}}, exp: false},
		{
			desc: "invalid rate option",
			param: &QueryParam{Start: 1, Queries: []SubQuery{{
				Aggregator: "sum", Metric: "cpu", RateParams: map[string]any{"unknown": true},
			}}},
			exp: false,
		},
		{
			desc: "valid rate options",
			param: &QueryParam{Start: "1h-ago", Queries: []SubQuery{{
				Aggregator: "sum", Metric: "cpu",
				RateParams: map[string]any{queryRateOptionCounter: true, queryRateOptionCounterMax: 10, queryRateOptionResetValue: 1},
			}}},
			exp: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.exp, isValidQueryParam(tc.param))
		})
	}
}

func TestIsValidTimePoint(t *testing.T) {
	tests := []struct {
		desc      string
		timePoint any
		exp       bool
	}{
		{desc: "nil", timePoint: nil, exp: false},
		{desc: "positive int", timePoint: 1, exp: true},
		{desc: "zero int", timePoint: 0, exp: false},
		{desc: "positive int64", timePoint: int64(5), exp: true},
		{desc: "negative int64", timePoint: int64(-1), exp: false},
		{desc: "non-empty string", timePoint: "1h-ago", exp: true},
		{desc: "empty string", timePoint: "", exp: false},
		{desc: "unsupported type", timePoint: 1.5, exp: false},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.exp, isValidTimePoint(tc.timePoint))
		})
	}
}

func TestIsValidQueryLastParam(t *testing.T) {
	tests := []struct {
		desc  string
		param *QueryLastParam
		exp   bool
	}{
		{desc: "no queries", param: &QueryLastParam{}, exp: false},
		{desc: "empty metric", param: &QueryLastParam{Queries: []SubQueryLast{{Metric: ""}}}, exp: false},
		{desc: "valid metric", param: &QueryLastParam{Queries: []SubQueryLast{{Metric: "cpu"}}}, exp: true},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.exp, isValidQueryLastParam(tc.param))
		})
	}
}

func TestInitializeClient(t *testing.T) {
	customTransport := &http.Transport{MaxIdleConnsPerHost: 99}

	tests := []struct {
		desc                string
		config              Config
		setupMocks          func(l *MockLogger)
		expHost             string
		expMaxPutPointsNum  int
		expDetectDeltaNum   int
		expMaxContentLength int
		expIdleConnsPerHost int
	}{
		{
			desc:   "empty host logs fatal and applies defaults",
			config: Config{Host: "   "},
			setupMocks: func(l *MockLogger) {
				l.EXPECT().Fatal("the OpenTSDB Endpoint in the given configuration cannot be empty.").Times(1)
			},
			expHost:             "",
			expMaxPutPointsNum:  defaultMaxPutPointsNum,
			expDetectDeltaNum:   defaultDetectDeltaNum,
			expMaxContentLength: defaultMaxContentLength,
			expIdleConnsPerHost: 10,
		},
		{
			desc: "custom transport and limits are kept",
			config: Config{
				Host: " localhost:4242 ", Transport: customTransport,
				MaxPutPointsNum: 5, DetectDeltaNum: 6, MaxContentLength: 7,
			},
			setupMocks:          func(*MockLogger) {},
			expHost:             "localhost:4242",
			expMaxPutPointsNum:  5,
			expDetectDeltaNum:   6,
			expMaxContentLength: 7,
			expIdleConnsPerHost: 99,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			mockLogger := NewMockLogger(gomock.NewController(t))
			tc.setupMocks(mockLogger)

			client := New(tc.config)
			client.UseLogger(mockLogger)

			client.initializeClient()

			httpClient, ok := client.client.(*http.Client)
			require.True(t, ok)

			transport, ok := httpClient.Transport.(*http.Transport)
			require.True(t, ok)

			assert.Equal(t, tc.expHost, client.config.Host)
			assert.Equal(t, tc.expMaxPutPointsNum, client.config.MaxPutPointsNum)
			assert.Equal(t, tc.expDetectDeltaNum, client.config.DetectDeltaNum)
			assert.Equal(t, tc.expMaxContentLength, client.config.MaxContentLength)
			assert.Equal(t, tc.expIdleConnsPerHost, transport.MaxIdleConnsPerHost)
		})
	}
}
