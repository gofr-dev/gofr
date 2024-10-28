package opentsdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
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

	parsedResp := AggregatorsResponse{logger: client.logger, tracer: client.tracer, ctx: client.ctx}

	err := client.sendRequest("GET", "http://localhost:4242/aggregators", "", &parsedResp)

	require.NoError(t, err)
	assert.Equal(t, 200, parsedResp.StatusCode)
	assert.Equal(t, []string{"sum", "avg"}, parsedResp.Aggregators)
}

func TestSendRequestFailure(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(nil, errors.New("request failed")).
		Times(1)

	parsedResp := AggregatorsResponse{logger: client.logger, tracer: client.tracer, ctx: client.ctx}

	err := client.sendRequest("GET", "http://localhost:4242/aggregators", "", &parsedResp)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "request failed")
}

func TestSetStatus(t *testing.T) {
	client, _ := setOpenTSDBTest(t)

	resp := &AggregatorsResponse{
		logger: client.logger,
		tracer: client.tracer,
		ctx:    client.ctx,
	}

	resp.SetStatus(200)

	assert.Equal(t, 200, resp.StatusCode)
}

func TestGetCustomParser(t *testing.T) {
	client, _ := setOpenTSDBTest(t)

	resp := &AggregatorsResponse{
		logger: client.logger,
		tracer: client.tracer,
		ctx:    client.ctx,
	}

	parser := resp.GetCustomParser()

	err := parser([]byte(`["sum","avg"]`))

	require.NoError(t, err)
	assert.Equal(t, []string{"sum", "avg"}, resp.Aggregators)
}

// setOpenTSDBTest initializes an Client for testing.
func setOpenTSDBTest(t *testing.T) (*Client, *MockHTTPClient) {
	t.Helper()

	opentsdbCfg := Config{
		Host:             "localhost:4242",
		MaxContentLength: 4096,
		MaxPutPointsNum:  1000,
		DetectDeltaNum:   10,
	}

	tsdbClient := New(&opentsdbCfg)

	tracer := otel.GetTracerProvider().Tracer("gofr-opentsdb")

	tsdbClient.UseTracer(tracer)

	mocklogger := NewMockLogger(gomock.NewController(t))

	tsdbClient.UseLogger(mocklogger)

	mocklogger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
	mocklogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mocklogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mocklogger.EXPECT().Log(gomock.Any()).AnyTimes()

	if tsdbClient.ctx == nil {
		tsdbClient.ctx = context.Background()
	}

	tsdbClient.config.Host = strings.TrimSpace(tsdbClient.config.Host)
	if tsdbClient.config.Host == "" {
		tsdbClient.logger.Errorf("the OpentsdbEndpoint in the given configuration cannot be empty.")
	}

	mockhttp := NewMockHTTPClient(gomock.NewController(t))

	tsdbClient.client = mockhttp

	// Set default values for optional configuration fields.
	if tsdbClient.config.MaxPutPointsNum <= 0 {
		tsdbClient.config.MaxPutPointsNum = DefaultMaxPutPointsNum
	}

	if tsdbClient.config.DetectDeltaNum <= 0 {
		tsdbClient.config.DetectDeltaNum = DefaultDetectDeltaNum
	}

	if tsdbClient.config.MaxContentLength <= 0 {
		tsdbClient.config.MaxContentLength = DefaultMaxContentLength
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
		data := DataPoint{
			Metric:    name[i%len(name)],
			Timestamp: time.Now().Unix(),
			Value:     rand.Float64() * 100,
			Tags:      tags,
		}
		cpuDatas = append(cpuDatas, data)
		t.Logf("Prepared datapoint %s\n", data.String())
	}

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"StatusCode":200,"failed":0,"success":4}`)),
		}, nil).Times(1)

	resp, err := client.Put(cpuDatas, "details")
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 200, resp.StatusCode)
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

	resp, err := client.Put(dataPoints, "")
	require.Error(t, err)
	require.Equal(t, "the value of the given datapoint is invalid", err.Error())
	require.Nil(t, resp)
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

	resp, err := client.Put(dataPoints, "invalid_param")
	require.Error(t, err)
	require.Equal(t, "The given query param is invalid.", err.Error())
	require.Nil(t, resp)
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

	resp, err := client.Put(dataPoints, "")
	require.Error(t, err)
	require.Equal(t, "Failed to put 0 datapoint(s) into opentsdb, statuscode 400:\n", err.Error())
	require.Nil(t, resp)
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

	queryResp, err := client.Query(&queryParam)
	require.NoError(t, err)
	require.NotNil(t, queryResp)
	require.Equal(t, 200, queryResp.StatusCode)

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

	queryLastResp, err := client.QueryLast(&queryLastParam)
	require.NoError(t, err)
	require.NotNil(t, queryLastResp)
	require.Equal(t, 200, queryLastResp.StatusCode)

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

	deleteResp, err := client.Query(&queryParam)
	require.NoError(t, err)
	require.NotNil(t, deleteResp)
	require.Equal(t, 200, deleteResp.StatusCode)
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

	aggreResp, err := client.Aggregators()
	require.NoError(t, err)
	require.NotNil(t, aggreResp)
	require.Equal(t, 200, aggreResp.StatusCode)

	var aggregators []string
	err = json.Unmarshal([]byte(expectedResponse), &aggregators)
	require.NoError(t, err)
	require.ElementsMatch(t, aggregators, aggreResp.Aggregators) // Assuming your response has an Aggregators field
}

func TestGetSuggestSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	expectedResponses := map[string]string{
		TypeMetrics: `["bytes","cpu","disk","mem","metric1","metric2","metric3","net","sys.cpu.0","sys.cpu.1"]`,
		TypeTagk:    `["demo-name","host","name","tag1","try-name","type"]`,
		TypeTagv:    `["bluebreezecf-host","bluebreezecf-sample","gofr-host","gofr-sample","opentsdb-test","value1","web01","web02","web03"]`,
	}

	for typeItem, expectedResponse := range expectedResponses {
		sugParam := SuggestParam{
			Type: typeItem,
		}

		mockHTTP.EXPECT().
			Do(gomock.Any()).
			Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(expectedResponse)),
			}, nil).Times(1)

		sugResp, err := client.Suggest(&sugParam)
		require.NoError(t, err)
		require.NotNil(t, sugResp)
		require.Equal(t, 200, sugResp.StatusCode)

		var suggestions []string
		err = json.Unmarshal([]byte(expectedResponse), &suggestions)
		require.NoError(t, err)
		require.ElementsMatch(t, suggestions, sugResp.ResultInfo) // Assuming your response has a Suggestions field
	}
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

	versionResp, err := client.version()
	require.NoError(t, err)
	require.NotNil(t, versionResp)
	require.Equal(t, 200, versionResp.StatusCode)

	var versionData struct {
		Version string `json:"version"`
	}

	err = json.Unmarshal([]byte(expectedResponse), &versionData)
	require.NoError(t, err)

	require.Equal(t, versionData.Version, versionResp.VersionInfo["version"])
}

func TestGetDropCachesSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	expectedResponse := `{"message":"Caches dropped","status":"200"}`

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(expectedResponse)), // Response body with message
		}, nil).Times(1)

	dropResp, err := client.Dropcaches()
	require.NoError(t, err)
	require.NotNil(t, dropResp)
	require.Equal(t, "Caches dropped", dropResp.DropCachesInfo["message"])
	require.Equal(t, 200, dropResp.StatusCode)
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

	queryAnnoResp, err := client.UpdateAnnotation(&anno)

	require.NoError(t, err)
	require.NotNil(t, queryAnnoResp)
	require.Equal(t, 200, queryAnnoResp.StatusCode)

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

	postResp, err := client.UpdateAnnotation(&anno)
	require.NoError(t, err)
	require.NotNil(t, postResp)
	require.Equal(t, 200, postResp.StatusCode)

	queryAnnoMap := map[string]interface{}{
		AnQueryStartTime: addedST,
		AnQueryTSUid:     addedTsuid,
	}

	resp, err := client.QueryAnnotation(queryAnnoMap)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 200, resp.StatusCode)
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

	postResp, err := client.UpdateAnnotation(&anno)
	require.NoError(t, err)
	require.NotNil(t, postResp)
	require.Equal(t, 200, postResp.StatusCode)

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusNoContent,
			Body:       http.NoBody,
		}, nil).Times(1)

	deleteResp, err := client.DeleteAnnotation(&anno)
	require.NoError(t, err)
	require.NotNil(t, deleteResp)
	require.Equal(t, 204, deleteResp.StatusCode)

	require.Empty(t, deleteResp.TSUID)
	require.Empty(t, deleteResp.StartTime)
	require.Empty(t, deleteResp.Description)
}

func TestBulkUpdateAnnotationsSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	anns := make([]Annotation, 0)
	bulkAnnNum := 4
	i := 0

	for {
		if !(i < bulkAnnNum-1) {
			break
		}

		addedST := time.Now().Unix()
		addedTsuid := fmt.Sprintf("%s%d", "00000100000100000", i)

		anno := Annotation{
			StartTime:   addedST,
			TSUID:       addedTsuid,
			Description: "gofr test annotation",
			Notes:       "These would be details about the event, the description is just a summary",
		}
		anns = append(anns, anno)
		i++
	}

	expectedResponse := `[{
		"tsuid":"000001000001000000",
		"description":"gofr test annotation",
		"notes":"These would be details about the event, the description is just a summary",
		"startTime":1728843948,
		"endTime":0
	},{
		"tsuid":"000001000001000001",
		"description":"gofr test annotation",
		"notes":"These would be details about the event, the description is just a summary",
		"startTime":1728843948,
		"endTime":0
	},{
		"tsuid":"000001000001000002",
		"description":"gofr test annotation",
		"notes":"These would be details about the event, the description is just a summary",
		"startTime":1728843948,
		"endTime":0
	}]`

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(expectedResponse)),
		}, nil).Times(1)

	resp, err := client.BulkUpdateAnnotations(anns)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 200, resp.StatusCode)

	var annotations []Annotation
	err = json.Unmarshal([]byte(expectedResponse), &annotations)
	require.NoError(t, err)
	require.Equal(t, annotations, resp.UpdateAnnotations)
}

func TestBulkDeleteAnnotationsSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)
	anns := make([]Annotation, 0)
	bulkAnnNum := 4
	i := 0
	bulkAddBeginST := time.Now().Unix()
	addedTsuids := make([]string, 0)

	for {
		if !(i < bulkAnnNum-1) {
			break
		}

		addedTsuid := fmt.Sprintf("%s%d", "00000100000100000", i)
		addedTsuids = append(addedTsuids, addedTsuid)

		anno := Annotation{
			StartTime:   bulkAddBeginST,
			TSUID:       addedTsuid,
			Description: "gofr test annotation",
			Notes:       "These would be details about the event, the description is just a summary",
		}
		anns = append(anns, anno)
		i++
	}

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`[{
				"tsuid": "000001000001000000",
				"description": "gofr test annotation",
				"notes": "These would be details about the event, the description is just a summary",
				"startTime": 1728844256,
				"endTime": 0
			},
			{
				"tsuid": "000001000001000001",
				"description": "gofr test annotation",
				"notes": "These would be details about the event, the description is just a summary",
				"startTime": 1728844256,
				"endTime": 0
			},
			{
				"tsuid": "000001000001000002",
				"description": "gofr test annotation",
				"notes": "These would be details about the event, the description is just a summary",
				"startTime": 1728844256,
				"endTime": 0
			}]`)),
		}, nil).Times(1)

	_, err := client.BulkUpdateAnnotations(anns)
	require.NoError(t, err)

	bulkAnnoDelete := BulkAnnotationDeleteInfo{
		StartTime: bulkAddBeginST,
		TSUIDs:    addedTsuids,
		Global:    false,
	}

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"tsuids": [
					"000001000001000000",
					"000001000001000001",
					"000001000001000002"
				],
				"global": false,
				"startTime": 1728844256000,
				"endTime": 1728844296169,
				"totalDeleted": 3
			}`)),
		}, nil).Times(1)

	resp, err := client.BulkDeleteAnnotations(&bulkAnnoDelete)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 200, resp.StatusCode)

	require.Equal(t, int64(3), resp.TotalDeleted)
	require.ElementsMatch(t, addedTsuids, resp.TSUIDs)
	require.False(t, resp.Global)
}

func TestAssignUIDSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	metrics := []string{"abcd"}
	tagk := []string{"gofr"}
	tagv := []string{"web7"}
	assignParam := UIDAssignParam{
		Metric: metrics,
		Tagk:   tagk,
		Tagv:   tagv,
	}

	expectedResponse := `{"metric":{"abcd":"00000B"},"tagk":{"gofr":"000007"},"tagv":{"web7":"00000A"}}`

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(expectedResponse)), // Response body with UIDs
		}, nil).Times(1)

	resp, err := client.AssignUID(&assignParam)

	require.NoError(t, err, "Error occurred while assigning UID info")
	require.NotEmpty(t, resp.Metric, "Expected metric to be nil") // This may need adjustment based on your response structure
	require.Empty(t, resp.MetricErrors, "Expected metric error to not be nil")

	expectedMetric := map[string]string{"abcd": "00000B"}
	expectedTagK := map[string]string{"gofr": "000007"}
	expectedTagV := map[string]string{"web7": "00000A"}

	require.Equal(t, expectedMetric, resp.Metric)
	require.Equal(t, expectedTagK, resp.Tagk)
	require.Equal(t, expectedTagV, resp.Tagv)
}

func TestUpdateUIDMetaDataSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	uidMetaData := UIDMetaData{
		UID:         "00000B",
		Type:        "metric",               // Type is "metric" (correct)
		DisplayName: "System CPU Time",      // Display name
		Description: "CPU usage for system", // Optional but helpful field
	}

	expectedResponse := `{"uid":"00000B","type":"METRIC","name":"abcd","description":"CPU usage for system",` +
		`"notes":"","created":0,"custom":null,"displayName":"System CPU Time"}`

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(expectedResponse)), // Mock response body
		}, nil).Times(1)

	resp, err := client.UpdateUIDMetaData(&uidMetaData)

	require.NoError(t, err, "Error occurred while posting UID metadata info")
	require.NotNil(t, resp, "Response should not be nil")

	require.Equal(t, "00000B", resp.UID, "UID should match")
	require.Equal(t, "METRIC", resp.Type, "Type should match")
	require.Equal(t, "abcd", resp.Name, "Name should match")
	require.Equal(t, "CPU usage for system", resp.Description, "Description should match")
	require.Equal(t, "System CPU Time", resp.DisplayName, "DisplayName should match")
}

func TestQueryUIDMetaDataSuccess(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	metaQueryParam := make(map[string]string, 0)
	metaQueryParam["type"] = TypeMetrics
	metaQueryParam["uid"] = "00003A"

	expectedResponse := `{"uid":"00000B","type":"METRIC","name":"abcd","description":` +
		`"CPU usage for system","notes":"","created":0,"custom":null,"displayName":"System CPU Time"}`

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(expectedResponse)),
		}, nil).Times(1)

	resp, err := client.QueryUIDMetaData(metaQueryParam)

	require.NoError(t, err, "Error occurred while querying uidmetadata info")
	require.NotNil(t, resp, "Response should not be nil")
	require.Equal(t, 200, resp.StatusCode)

	var uidMetaData map[string]interface{}
	err = json.Unmarshal([]byte(expectedResponse), &uidMetaData)
	require.NoError(t, err)
	require.Equal(t, uidMetaData["uid"], resp.UID)
	require.Equal(t, uidMetaData["name"], resp.Name)
}

func TestDeleteUIDMetaData(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	uidMetaData := UIDMetaData{
		UID:  "00000B",
		Type: "metric",
	}

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("[]")),
		}, nil).Times(1)

	resp, err := client.DeleteUIDMetaData(&uidMetaData)

	require.NoError(t, err, "Error occurred while deleting UID metadata")
	require.NotNil(t, resp, "Response should not be nil")
	require.Equal(t, 204, resp.StatusCode, "Unexpected status code, expected 204 for successful deletion")
}

func TestUpdateTSMetaData(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	custom := make(map[string]string)
	custom["owner"] = "gofr"
	custom["department"] = "framework"

	tsMetaData := TSMetaData{
		TSUID:       "00002A000001000001",
		Description: "This timeseries represents the system CPU time for webserver 01.",
		DisplayName: "Webserver 01 CPU Time",
		Notes:       "Monitored for performance analysis and resource allocation.",
		Custom:      custom,
	}

	expectedResponse := `{"error":{"message":"unknown Tsuid"}}`

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(expectedResponse)),
		}, nil).
		Times(1)

	resp, err := client.UpdateTSMetaData(&tsMetaData)

	require.NoError(t, err, "Error occurred while posting TS metadata")
	require.NotNil(t, resp, "Response should not be nil")

	require.Equal(t, 500, resp.StatusCode, "Unexpected status code, expected 500 for unsuccessful update")
	require.NotEmpty(t, resp.ErrorInfo, "Expected error to be not nil")
}

func TestQueryTSMetaData(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	tsuid := "000000B"

	expectedResponse := `{"error":{"message":"tsuid not found"}}`

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(expectedResponse)),
		}, nil).Times(1)

	resp, err := client.QueryTSMetaData(tsuid)

	require.NoError(t, err, "Error occurred while querying TS metadata")
	require.Empty(t, resp.Metric, "Metric should be nil")
	require.NotEmpty(t, resp.ErrorInfo, "ErrorInfo should be not nil")
	require.Equal(t, 404, resp.StatusCode, "Unexpected status code, expected 404 for invalid tsuid")
}

func TestDeleteTSMetaData(t *testing.T) {
	client, mockHTTP := setOpenTSDBTest(t)

	tsMetaData := TSMetaData{
		TSUID: "000001000001000001",
	}

	mockHTTP.EXPECT().
		Do(gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil).Times(1)

	resp, err := client.DeleteTSMetaData(&tsMetaData)

	require.NoError(t, err, "Error occurred while deleting TS metadata")
	require.NotNil(t, resp, "Response should not be nil")

	require.Equal(t, 204, resp.StatusCode, "Unexpected status code, expected 204 for successful deletion")
}

func TestGetDataPoints(t *testing.T) {
	client, _ := setOpenTSDBTest(t)

	qri := &QueryRespItem{
		Metric: "cpu_usage",
		Tags:   map[string]string{"host": "server1"},
		Dps: map[string]interface{}{
			"1609459200": 0.5,
			"1609459260": 0.6,
			"1609459320": 0.7,
		},
		logger: client.logger,
		tracer: client.tracer,
		ctx:    client.ctx,
	}

	dataPoints := qri.GetDataPoints()

	require.Len(t, dataPoints, 3, "Expected 3 datapoints")
	require.Equal(t, "cpu_usage", dataPoints[0].Metric, "Metric should match")
	require.Equal(t, int64(1609459200), dataPoints[0].Timestamp, "Timestamp should match")
	require.InEpsilon(t, 0.5, dataPoints[0].Value, 0, "Value should match")
}

func TestGetSortedTimestampStrs(t *testing.T) {
	client, _ := setOpenTSDBTest(t)

	qri := &QueryRespItem{
		Dps: map[string]interface{}{
			"1609459260": 0.6,
			"1609459200": 0.5,
			"1609459320": 0.7,
		},
		logger: client.logger,
		tracer: client.tracer,
		ctx:    client.ctx,
	}

	timestampStrs := qri.getSortedTimestampStrs()

	require.Len(t, timestampStrs, 3, "Expected 3 timestamps")
	require.Equal(t, []string{"1609459200", "1609459260", "1609459320"}, timestampStrs, "Timestamps should be sorted")
}

func TestGetLatestDataPoint(t *testing.T) {
	client, _ := setOpenTSDBTest(t)

	qri := &QueryRespItem{
		Metric: "cpu_usage",
		Tags:   map[string]string{"host": "server1"},
		Dps:    map[string]interface{}{},
		logger: client.logger,
		tracer: client.tracer,
		ctx:    client.ctx,
	}

	latestDataPoint := qri.GetLatestDataPoint()

	require.Nil(t, latestDataPoint, "Expected no latest datapoint")

	qri.Dps["1609459200"] = 0.5
	qri.Dps["1609459260"] = 0.6
	qri.Dps["1609459320"] = 0.7

	latestDataPoint = qri.GetLatestDataPoint()

	require.NotNil(t, latestDataPoint, "Expected a latest datapoint")
	require.Equal(t, "cpu_usage", latestDataPoint.Metric, "Metric should match")
	require.Equal(t, int64(1609459320), latestDataPoint.Timestamp, "Timestamp should match")
	require.InEpsilonf(t, 0.7, latestDataPoint.Value, 0, "Value should match")
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

	mockConn := NewMockConn(gomock.NewController(t))

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
		return nil, errors.New("connection error")
	}

	resp, err := client.HealthCheck(context.Background())

	require.Error(t, err, "Expected error during health check")
	require.Nil(t, resp, "Expected response to be nil")
	require.Equal(t, "OpenTSDB is unreachable: connection error", err.Error(), "Expected specific error message")
}
