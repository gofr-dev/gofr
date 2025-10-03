package opentsdb

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
)

var (
	errConnection    = errors.New("connection error")
	errRequestFailed = errors.New("request failed")
)

func TestSendRequestSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	mockResponse := http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`["sum","avg"]`)),
	}

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&mockResponse, nil).
		Times(1)

	parsedResp := AggregatorsResponse{}

	err := client.sendRequest(context.Background(), "GET", "http://localhost:4242/aggregators", "", &parsedResp)

	require.NoError(t, err)
	assert.Equal(t, []string{"sum", "avg"}, parsedResp.Aggregators)
}

func TestSendRequestFailure(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(nil, errRequestFailed).
		Times(1)

	parsedResp := AggregatorsResponse{}

	err := client.sendRequest(context.Background(), "GET", "http://localhost:4242/aggregators", "", &parsedResp)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "request failed")
}

func TestGetCustomParser(t *testing.T) {
	client, _ := setOpenTSDBTest(t)

	resp := &AggregatorsResponse{}

	parser := resp.getCustomParser(client.logger)

	err := parser([]byte(`["sum","avg"]`))

	require.NoError(t, err)
	assert.Equal(t, []string{"sum", "avg"}, resp.Aggregators)
}

// setOpenTSDBTest initializes an Client for testing.
func setOpenTSDBTest(t *testing.T) (*Client, *MockhttpClient) {
	t.Helper()

	opentsdbCfg := Config{
		Host:             "localhost:4242",
		MaxContentLength: 4096,
		MaxPutPointsNum:  1000,
		DetectDeltaNum:   10,
	}

	tsdbClient := New(opentsdbCfg)

	tracer := otel.GetTracerProvider().Tracer("gofr-opentsdb")

	tsdbClient.UseTracer(tracer)

	mocklogger := NewMockLogger(gomock.NewController(t))

	tsdbClient.UseLogger(mocklogger)

	mocklogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocklogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocklogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mocklogger.EXPECT().Log(gomock.Any()).AnyTimes()

	tsdbClient.config.Host = strings.TrimSpace(tsdbClient.config.Host)
	if tsdbClient.config.Host == "" {
		tsdbClient.logger.Errorf("the OpentsdbEndpoint in the given configuration cannot be empty.")
	}

	mockhttp := NewMockhttpClient(gomock.NewController(t))

	tsdbClient.client = mockhttp

	// Set default values for optional configuration fields.
	if tsdbClient.config.MaxPutPointsNum <= 0 {
		tsdbClient.config.MaxPutPointsNum = defaultMaxPutPointsNum
	}

	if tsdbClient.config.DetectDeltaNum <= 0 {
		tsdbClient.config.DetectDeltaNum = defaultDetectDeltaNum
	}

	if tsdbClient.config.MaxContentLength <= 0 {
		tsdbClient.config.MaxContentLength = defaultMaxContentLength
	}

	// Initialize the OpenTSDB client with the given configuration.
	tsdbClient.endpoint = fmt.Sprintf("http://%s", tsdbClient.config.Host)

	return tsdbClient, mockhttp
}

func TestPutSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	PutDataPointNum := 4
	name := []string{"cpu", "disk", "net", "mem"}
	cpuDatas := make([]DataPoint, 0)

	tags := map[string]string{
		"host":      "gofr-host",
		"try-name":  "gofr-sample",
		"demo-name": "opentsdb-test",
	}

	for i := 0; i < PutDataPointNum; i++ {
		val, _ := rand.Int(rand.Reader, big.NewInt(100))
		data := DataPoint{
			Metric:    name[i%len(name)],
			Timestamp: time.Now().Unix(),
			Value:     val.Int64(),
			Tags:      tags,
		}
		cpuDatas = append(cpuDatas, data)
	}

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"StatusCode":200,"failed":0,"success":4}`)),
		}, nil).Times(1)

	resp := &PutResponse{}

	err := client.PutDataPoints(context.Background(), cpuDatas, "details", resp)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, int64(len(cpuDatas)), resp.Success)
}

func TestPutInvalidDataPoint(t *testing.T) {
	client, _ := setOpenTSDBTest(t)

	dataPoints := []DataPoint{
		{
			Metric:    "",
			Timestamp: 0,
			Value:     0,
			Tags:      map[string]string{},
		},
	}

	resp := &PutResponse{}

	err := client.PutDataPoints(context.Background(), dataPoints, "", resp)
	require.ErrorIs(t, err, errInvalidDataPoint)
	require.ErrorContains(t, err, "please give a valid value")
}

func TestPutInvalidQueryParam(t *testing.T) {
	client, _ := setOpenTSDBTest(t)

	dataPoints := []DataPoint{
		{
			Metric:    "metric1",
			Timestamp: time.Now().Unix(),
			Value:     100,
			Tags:      map[string]string{"tag1": "value1"},
		},
	}

	resp := &PutResponse{}

	err := client.PutDataPoints(context.Background(), dataPoints, "invalid_param", resp)
	require.ErrorIs(t, err, errInvalidQueryParam)
}

func TestPutErrorResponse(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	dataPoints := []DataPoint{
		{
			Metric:    "invalid_metric_name#$%",
			Timestamp: time.Now().Unix(),
			Value:     100,
			Tags:      map[string]string{"tag1": "value1"},
		},
	}

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader(`{"StatusCode":400,"error":"Invalid metric name"}`)),
		}, nil).Times(1)

	resp := &PutResponse{}

	err := client.PutDataPoints(context.Background(), dataPoints, "", resp)
	require.ErrorIs(t, err, errResp)
	require.ErrorContains(t, err, "status code: 400")
}

func TestPostQuerySuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	st1 := time.Now().Unix() - 3600
	st2 := time.Now().Unix()
	queryParam := QueryParam{
		Start: st1,
		End:   st2,
	}

	name := []string{"cpu", "disk", "net", "mem"}
	subqueries := make([]SubQuery, 0)
	tags := map[string]string{
		"host":      "gofr-host",
		"try-name":  "gofr-sample",
		"demo-name": "opentsdb-test",
	}

	for _, metric := range name {
		subQuery := SubQuery{
			Aggregator: "sum",
			Metric:     metric,
			Tags:       tags,
		}
		subqueries = append(subqueries, subQuery)
	}

	queryParam.Queries = subqueries

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`[{"metric":"net","timestamp":1728836485000,"value":` +
				`"19.499737232159088","tags":{"demo-name":"opentsdb-test","host":"gofr-host","try-name":` +
				`"gofr-sample"},"tsuid":"000003000001000001000002000007000003000008"},{"metric":"disk",` +
				`"timestamp":1728836485000,"value":"98.53097270356102","tags":{"demo-name":"opentsdb-test",` +
				`"host":"gofr-host","try-name":"gofr-sample"},"tsuid":"000002000001000001000002000007000003000008"}` +
				`,{"metric":"cpu","timestamp":1728836485000,"value":"49.47446557839882","tags":{"demo-name":"opentsdb` +
				`-test","host":"gofr-host","try-name":"gofr-sample"},"tsuid":"000001000001000001000002000007000003000008"}` +
				`,{"metric":"mem","timestamp":1728836485000,"value":"28.62340008609452","tags":{"demo-name":"opentsdb-test",` +
				`"host":"gofr-host","try-name":"gofr-sample"},"tsuid":"000004000001000001000002000007000003000008"}]`)),
		}, nil).Times(1)

	queryResp := &QueryResponse{}
	err := client.QueryDataPoints(context.Background(), &queryParam, queryResp)
	require.NoError(t, err)
	require.NotNil(t, queryResp)

	require.Len(t, queryResp.QueryRespCnts, 4)
}

func TestPostQueryLastSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	name := []string{"cpu", "disk", "net", "mem"}
	subqueriesLast := make([]SubQueryLast, 0)
	tags := map[string]string{
		"host":      "gofr-host",
		"try-name":  "gofr-sample",
		"demo-name": "opentsdb-test",
	}

	for _, metric := range name {
		subQueryLast := SubQueryLast{
			Metric: metric,
			Tags:   tags,
		}
		subqueriesLast = append(subqueriesLast, subQueryLast)
	}

	queryLastParam := QueryLastParam{
		Queries:      subqueriesLast,
		ResolveNames: true,
		BackScan:     24,
	}

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`[{"metric":"net","timestamp":1728836485000,"value":` +
				`"19.499737232159088","tags":{"demo-name":"opentsdb-test","host":"gofr-host","try-name":` +
				`"gofr-sample"},"tsuid":"000003000001000001000002000007000003000008"},{"metric":"disk",` +
				`"timestamp":1728836485000,"value":"98.53097270356102","tags":{"demo-name":"opentsdb-test",` +
				`"host":"gofr-host","try-name":"gofr-sample"},"tsuid":"000002000001000001000002000007000003000008"}` +
				`,{"metric":"cpu","timestamp":1728836485000,"value":"49.47446557839882","tags":{"demo-name":"opentsdb` +
				`-test","host":"gofr-host","try-name":"gofr-sample"},"tsuid":"000001000001000001000002000007000003000008"}` +
				`,{"metric":"mem","timestamp":1728836485000,"value":"28.62340008609452","tags":{"demo-name":"opentsdb-test",` +
				`"host":"gofr-host","try-name":"gofr-sample"},"tsuid":"000004000001000001000002000007000003000008"}]`)),
		}, nil).Times(1)

	queryLastResp := &QueryLastResponse{}

	err := client.QueryLatestDataPoints(context.Background(), &queryLastParam, queryLastResp)
	require.NoError(t, err)
	require.NotNil(t, queryLastResp)

	require.Len(t, queryLastResp.QueryRespCnts, 4)
}

func TestPostQueryDeleteSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	st1 := time.Now().Unix() - 3600
	st2 := time.Now().Unix()
	queryParam := QueryParam{
		Start:  st1,
		End:    st2,
		Delete: true,
	}

	name := []string{"cpu", "disk", "net", "mem"}
	subqueries := make([]SubQuery, 0)
	tags := map[string]string{
		"host":      "gofr-host",
		"try-name":  "gofr-sample",
		"demo-name": "opentsdb-test",
	}

	for _, metric := range name {
		subQuery := SubQuery{
			Aggregator: "sum",
			Metric:     metric,
			Tags:       tags,
		}
		subqueries = append(subqueries, subQuery)
	}

	queryParam.Queries = subqueries

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`[]`)),
		}, nil).Times(1)

	deleteResp := &QueryResponse{}

	err := client.QueryDataPoints(context.Background(), &queryParam, deleteResp)
	require.NoError(t, err)
	require.NotNil(t, deleteResp)
}

func TestGetAggregatorsSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	expectedResponse := `["sum","avg","max","min","count"]`

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(expectedResponse)), // Response body with aggregators
		}, nil).Times(1)

	aggreResp := &AggregatorsResponse{}

	err := client.GetAggregators(context.Background(), aggreResp)
	require.NoError(t, err)
	require.NotNil(t, aggreResp)

	var aggregators []string

	err = json.Unmarshal([]byte(expectedResponse), &aggregators)
	require.NoError(t, err)
	require.ElementsMatch(t, aggregators, aggreResp.Aggregators) // Assuming your response has an Aggregators field
}

func TestGetVersionSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	expectedResponse := `{"short_revision":"","repo":"/opt/opentsdb/opentsdb-2.4.0/build",` +
		`"host":"a0d1ce2d1fd7","version":"2.4.0","full_revision":"","repo_status":"MODIFIED"` +
		`,"user":"root","branch":"","timestamp":"1607178614"}`

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(expectedResponse)), // Response body for version
		}, nil).Times(1)

	versionResp := &VersionResponse{}

	err := client.version(context.Background(), versionResp)
	require.NoError(t, err)
	require.NotNil(t, versionResp)

	var versionData struct {
		Version string `json:"version"`
	}

	err = json.Unmarshal([]byte(expectedResponse), &versionData)
	require.NoError(t, err)

	require.Equal(t, versionData.Version, versionResp.VersionInfo["version"])
}

func TestUpdateAnnotationSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	custom := map[string]string{
		"owner": "gofr",
		"host":  "gofr-host",
	}
	addedST := time.Now().Unix()
	addedTsuid := "000001000001000002"
	anno := Annotation{
		StartTime:   addedST,
		TSUID:       addedTsuid,
		Description: "gofrf test annotation",
		Notes:       "These would be details about the event, the description is just a summary",
		Custom:      custom,
	}

	expectedResponse := `{"tsuid":"000001000001000002","description":"gofrf test annotation","notes":` +
		`"These would be details about the event, the description is just a summary","custom":{"host":` +
		`"gofr-host","owner":"gofr"},"startTime":` + fmt.Sprintf("%d", addedST) + `,"endTime":0}`

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(expectedResponse)), // Response body for the annotation
		}, nil).Times(1)

	queryAnnoResp := &AnnotationResponse{}

	err := client.PostAnnotation(context.Background(), &anno, queryAnnoResp)

	require.NoError(t, err)
	require.NotNil(t, queryAnnoResp)

	require.Equal(t, anno.TSUID, queryAnnoResp.TSUID)
	require.Equal(t, anno.StartTime, queryAnnoResp.StartTime)
	require.Equal(t, anno.Description, queryAnnoResp.Description)
	require.Equal(t, anno.Notes, queryAnnoResp.Notes)
	require.Equal(t, anno.Custom, queryAnnoResp.Custom)
}

func TestQueryAnnotationSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	custom := map[string]string{
		"owner": "gofr",
		"host":  "gofr-host",
	}
	addedST := time.Now().Unix()
	addedTsuid := "000001000001000002"
	anno := Annotation{
		StartTime:   addedST,
		TSUID:       addedTsuid,
		Description: "gofr test annotation",
		Notes:       "These would be details about the event, the description is just a summary",
		Custom:      custom,
	}

	mockResponse := `{"tsuid":"000001000001000002","description":"gofr test annotation","notes":"These` +
		` would be details about the event, the description is just a summary","custom":{"host"` +
		`:"gofr-host","owner":"gofr"},"startTime":1728841614,"endTime":0}`

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		DoAndReturn(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(mockResponse)), // Fresh body each time
			}, nil
		}).Times(2)

	postResp := &AnnotationResponse{}

	err := client.PostAnnotation(context.Background(), &anno, postResp)
	require.NoError(t, err)
	require.NotNil(t, postResp)

	queryAnnoMap := map[string]any{
		anQueryStartTime: addedST,
		anQueryTSUid:     addedTsuid,
	}

	queryResp := &AnnotationResponse{}

	err = client.QueryAnnotation(context.Background(), queryAnnoMap, queryResp)
	require.NoError(t, err)
	require.NotNil(t, queryResp)
}

func TestDeleteAnnotationSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	custom := map[string]string{
		"owner": "gofr",
		"host":  "gofr-host",
	}
	addedST := time.Now().Unix()
	addedTsuid := "000001000001000002"
	anno := Annotation{
		StartTime:   addedST,
		TSUID:       addedTsuid,
		Description: "gofr-host test annotation",
		Notes:       "These would be details about the event, the description is just a summary",
		Custom:      custom,
	}

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"tsuid": "000001000001000002",
				"description": "gofr-host test annotation",
				"notes": "These would be details about the event, the description is just a summary",
				"custom": {"host": "gofr-host", "owner": "gofr"},
				"startTime": 1728843749,
				"endTime": 0
			}`)),
		}, nil).Times(1)

	postResp := &AnnotationResponse{}

	err := client.PostAnnotation(context.Background(), &anno, postResp)
	require.NoError(t, err)
	require.NotNil(t, postResp)

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusNoContent,
			Body:       http.NoBody,
		}, nil).Times(1)

	deleteResp := &AnnotationResponse{}

	err = client.DeleteAnnotation(context.Background(), &anno, deleteResp)
	require.NoError(t, err)
	require.NotNil(t, deleteResp)

	require.Empty(t, deleteResp.TSUID)
	require.Empty(t, deleteResp.StartTime)
	require.Empty(t, deleteResp.Description)
}

func TestHealthCheck_Success(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"version": "2.9.0"}`)),
		}, nil).
		Times(1)

	mockConn := NewMockconnection(gomock.NewController(t))

	mockConn.EXPECT().Close()

	dialTimeout = func(_, _ string, _ time.Duration) (net.Conn, error) {
		return mockConn, nil
	}

	resp, err := client.HealthCheck(context.Background())

	require.NoError(t, err, "Expected no error during health check")
	require.NotNil(t, resp, "Expected response to be not nil")
	require.Equal(t, "UP", resp.(*Health).Status, "Expected status to be UP")
	require.Equal(t, "2.9.0", resp.(*Health).Details["version"], "Expected version to be 2.9.0")
}

func TestHealthCheck_Failure(t *testing.T) {
	client, _ := setOpenTSDBTest(t)

	dialTimeout = func(_, _ string, _ time.Duration) (net.Conn, error) {
		return nil, errConnection
	}

	resp, err := client.HealthCheck(context.Background())

	require.Nil(t, resp, "Expected response to be nil")
	require.EqualError(t, err, "connection error", "Expected error during health check")
}
