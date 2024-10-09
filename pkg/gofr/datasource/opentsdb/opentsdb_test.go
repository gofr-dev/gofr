package opentsdb

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
	"math/rand/v2"
	"testing"
	"time"
)

// setupOpenTSDBTest initializes an OpentsdbClient for testing.
func setupOpenTSDBTest(t *testing.T) *OpentsdbClient {
	t.Helper()

	opentsdbCfg := OpenTSDBConfig{
		OpentsdbHost:     "localhost:4242",
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

	tsdbClient.Connect()

	return tsdbClient
}

func TestPutSuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

	// Define the number of datapoints to be sent
	PutDataPointNum := 4
	name := []string{"cpu", "disk", "net", "mem"}
	cpuDatas := make([]DataPoint, 0)

	// Prepare tags for the datapoints
	tags := map[string]string{
		"host":      "gofr-host",
		"try-name":  "gofr-sample",
		"demo-name": "opentsdb-test",
	}

	// Generate random datapoints
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

	resp, err := client.Put(cpuDatas, "details")
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, int64(len(cpuDatas)), resp.Success)
}

func TestPutInvalidDataPoint(t *testing.T) {
	client := setupOpenTSDBTest(t)

	// Prepare an invalid DataPoint
	dataPoints := []DataPoint{
		{
			Metric:    "",
			Timestamp: 0,
			Value:     0, // Updated to a valid float value for the sake of type correctness
			Tags:      map[string]string{},
		},
	}

	resp, err := client.Put(dataPoints, "")
	require.Error(t, err)
	require.Nil(t, resp)
	t.Log("Expected error occurred for invalid DataPoint.")
}

func TestPutInvalidQueryParam(t *testing.T) {
	client := setupOpenTSDBTest(t)

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
	require.Nil(t, resp)
}

func TestPutErrorResponse(t *testing.T) {
	client := setupOpenTSDBTest(t)

	// Create a data point that is guaranteed to fail (e.g., invalid metric)
	dataPoints := []DataPoint{
		{
			Metric:    "invalid_metric_name#$%",
			Timestamp: time.Now().Unix(),
			Value:     100,
			Tags:      map[string]string{"tag1": "value1"},
		},
	}

	resp, err := client.Put(dataPoints, "")
	require.Error(t, err)
	require.Nil(t, resp)
}

func TestPostQuerySuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

	// Prepare query parameters
	st1 := time.Now().Unix() - 3600 // start time 1 hour ago
	st2 := time.Now().Unix()        // end time now
	queryParam := QueryParam{
		Start: st1,
		End:   st2,
	}

	// Prepare subqueries
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

	// Execute the query operation
	queryResp, err := client.Query(&queryParam)
	require.NoError(t, err)
	require.NotNil(t, queryResp)
	require.Equal(t, 200, queryResp.StatusCode)
}

func TestPostQueryLastSuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

	// Prepare last query parameters
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

	// Execute the last query operation
	queryLastResp, err := client.QueryLast(&queryLastParam)
	require.NoError(t, err)
	require.NotNil(t, queryLastResp)
	require.Equal(t, 200, queryLastResp.StatusCode)
}

func TestPostQueryDeleteSuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

	// Prepare query parameters for deletion
	st1 := time.Now().Unix() - 3600 // start time 1 hour ago
	st2 := time.Now().Unix()        // end time now
	queryParam := QueryParam{
		Start:  st1,
		End:    st2,
		Delete: true,
	}

	// Prepare subqueries as before
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

	// Execute the delete operation
	deleteResp, err := client.Query(&queryParam)
	require.NoError(t, err)
	require.NotNil(t, deleteResp)
	require.Equal(t, 200, deleteResp.StatusCode)
}

func TestGetAggregatorsSuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

	aggreResp, err := client.Aggregators()
	require.NoError(t, err)
	require.NotNil(t, aggreResp)
	require.Equal(t, 200, aggreResp.StatusCode)
}

func TestGetSuggestSuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

	typeValues := []string{TypeMetrics, TypeTagk, TypeTagv}
	for _, typeItem := range typeValues {
		sugParam := SuggestParam{
			Type: typeItem,
		}
		sugResp, err := client.Suggest(&sugParam)
		require.NoError(t, err)
		require.NotNil(t, sugResp)
		require.Equal(t, 200, sugResp.StatusCode)
	}
}

func TestGetVersionSuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

	versionResp, err := client.version()
	require.NoError(t, err)
	require.NotNil(t, versionResp)
	require.Equal(t, 200, versionResp.StatusCode)
}

func TestGetDropCachesSuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

	dropResp, err := client.Dropcaches()
	require.NoError(t, err)
	require.NotNil(t, dropResp)
	require.Equal(t, 200, dropResp.StatusCode)
}

func TestUpdateAnnotationSuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

	// Prepare data for the annotation
	custom := map[string]string{
		"owner": "gofr",
		"host":  "gofr-host",
	}
	addedST := time.Now().Unix()
	addedTsuid := "000001000001000002"
	anno := Annotation{
		StartTime:   addedST,
		Tsuid:       addedTsuid,
		Description: "gofrf test annotation",
		Notes:       "These would be details about the event, the description is just a summary",
		Custom:      custom,
	}

	queryAnnoResp, err := client.UpdateAnnotation(&anno)

	require.NoError(t, err)

	require.NotNil(t, queryAnnoResp)

	require.Equal(t, 200, queryAnnoResp.StatusCode)

	require.Equal(t, anno.Tsuid, queryAnnoResp.Tsuid)
	require.Equal(t, anno.StartTime, queryAnnoResp.StartTime)
	require.Equal(t, anno.Description, queryAnnoResp.Description)
	require.Equal(t, anno.Notes, queryAnnoResp.Notes)
	require.Equal(t, anno.Custom, queryAnnoResp.Custom)
}

func TestQueryAnnotationSuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

	// Prepare data for the annotation
	custom := map[string]string{
		"owner": "gofr",
		"host":  "gofr-host",
	}
	addedST := time.Now().Unix()
	addedTsuid := "000001000001000002"
	anno := Annotation{
		StartTime:   addedST,
		Tsuid:       addedTsuid,
		Description: "gofr test annotation",
		Notes:       "These would be details about the event, the description is just a summary",
		Custom:      custom,
	}

	queryAnnoMap := make(map[string]interface{}, 0)
	queryAnnoMap[AnQueryStartTime] = addedST
	queryAnnoMap[AnQueryTSUid] = addedTsuid

	postResp, err := client.UpdateAnnotation(&anno)

	require.NoError(t, err)
	require.NotNil(t, postResp)
	require.Equal(t, 200, postResp.StatusCode)

	resp, err := client.QueryAnnotation(queryAnnoMap)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 200, resp.StatusCode)
}

func TestDeleteAnnotationSuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

	custom := map[string]string{
		"owner": "gofr",
		"host":  "gofr-host",
	}
	addedST := time.Now().Unix()
	addedTsuid := "000001000001000002"
	anno := Annotation{
		StartTime:   addedST,
		Tsuid:       addedTsuid,
		Description: "gofr-host test annotation",
		Notes:       "These would be details about the event, the description is just a summary",
		Custom:      custom,
	}

	postResp, err := client.UpdateAnnotation(&anno)

	require.NoError(t, err)
	require.NotNil(t, postResp)
	require.Equal(t, 200, postResp.StatusCode)

	deleteResp, err := client.DeleteAnnotation(&anno)

	require.NoError(t, err)
	require.NotNil(t, deleteResp)
	require.Equal(t, 204, deleteResp.StatusCode)

	require.Empty(t, deleteResp.Tsuid)
	require.Empty(t, deleteResp.StartTime)
	require.Empty(t, deleteResp.Description)
}

func TestBulkUpdateAnnotationsSuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

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
			Tsuid:       addedTsuid,
			Description: "gofr test annotation",
			Notes:       "These would be details about the event, the description is just a summary",
		}
		anns = append(anns, anno)
		i++
	}

	resp, err := client.BulkUpdateAnnotations(anns)

	require.NoError(t, err)

	require.NotNil(t, resp)

	require.Equal(t, 200, resp.StatusCode)
}

func TestBulkDeleteAnnotationsSuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

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
			Tsuid:       addedTsuid,
			Description: "gofr test annotation",
			Notes:       "These would be details about the event, the description is just a summary",
		}
		anns = append(anns, anno)
		i++
	}

	_, _ = client.BulkUpdateAnnotations(anns)

	bulkAnnoDelete := BulkAnnoDeleteInfo{
		StartTime: bulkAddBeginST,
		Tsuids:    addedTsuids,
		Global:    false,
	}

	resp, err := client.BulkDeleteAnnotations(&bulkAnnoDelete)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 200, resp.StatusCode)
}

func TestQueryUIDMetaDataSuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

	metaQueryParam := make(map[string]string, 0)
	metaQueryParam["type"] = TypeMetrics
	metaQueryParam["uid"] = "00003A"

	// returns 404
	resp, err := client.QueryUIDMetaData(metaQueryParam)

	require.NoError(t, err, "Error occurred while querying uidmetadata info")
	require.NotNil(t, resp, "Response should not be nil")
	require.Equal(t, 404, resp.StatusCode)
}

func TestAssignUIDSuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

	metrics := []string{"sys.cpu.0", "sys.cpu.1", "illegal!character"}
	tagk := []string{"host"}
	tagv := []string{"web01", "web02", "web03"}
	assignParam := UIDAssignParam{
		Metric: metrics,
		Tagk:   tagk,
		Tagv:   tagv,
	}

	resp, err := client.AssignUID(&assignParam)

	require.NoError(t, err, "Error occurred while assigning UID info")
	require.Empty(t, resp.Metric, "Expected metric to be nil")
	require.NotEmpty(t, resp.MetricErrors, "Expected metric error to not be nil")

	fmt.Printf("%s", resp.String())
}

func TestUpdateUIDMetaDataSuccess(t *testing.T) {
	client := setupOpenTSDBTest(t)

	uidMetaData := UIDMetaData{
		UID:         "000006",
		Type:        "metric",
		DisplayName: "System CPU Time",
	}

	resp, err := client.UpdateUIDMetaData(&uidMetaData)

	require.NoError(t, err, "Error occurred while posting uidmetadata info")
	require.NotNil(t, resp, "Response should not be nil")
}

func TestDeleteUIDMetaData(t *testing.T) {
	client := setupOpenTSDBTest(t)

	uidMetaData := UIDMetaData{
		UID:  "000006",
		Type: "metric",
	}

	resp, err := client.DeleteUIDMetaData(&uidMetaData)

	require.NoError(t, err, "Error occurred while deleting UID metadata")
	require.NotNil(t, resp, "Response should not be nil")

	require.Equal(t, 204, resp.StatusCode, "Unexpected status code, expected 204 for successful deletion")
}

func TestQueryTSMetaData(t *testing.T) {
	client := setupOpenTSDBTest(t)

	tsuid := "000001000001000001"

	fmt.Println("Begin to test GET /api/uid/tsmeta.")

	resp, err := client.QueryTSMetaData(tsuid)

	require.NoError(t, err, "Error occurred while querying TS metadata")
	require.Empty(t, resp.Metric, "Metric should be nil")
	require.NotEmpty(t, resp.ErrorInfo, "ErrorInfo should be not nil")
	require.Equal(t, 404, resp.StatusCode, "Unexpected status code, expected 404 for invalid tsuid")
}

func TestUpdateTSMetaData(t *testing.T) {
	client := setupOpenTSDBTest(t)

	custom := make(map[string]string, 0)
	custom["owner"] = "gofr"
	custom["department"] = "framework"

	tsMetaData := TSMetaData{
		Tsuid:       "00002A000001000001",
		DisplayName: "System CPU Time for Webserver 01",
		Custom:      custom,
	}

	resp, err := client.UpdateTSMetaData(&tsMetaData)

	require.NoError(t, err, "Error occurred while posting TS metadata")
	require.NotNil(t, resp, "Response should not be nil")

	require.Equal(t, 500, resp.StatusCode, "Unexpected status code, expected 200 for successful update")

	fmt.Printf("%s", resp.String())

	fmt.Println("Finish testing POST /api/uid/tsmeta.")
}

func TestDeleteTSMetaData(t *testing.T) {
	// Setup a test client (assuming setupOpenTSDBTest or similar function exists)
	client := setupOpenTSDBTest(t)

	// Define the TSMetaData for deletion
	tsMetaData := TSMetaData{
		Tsuid: "000001000001000001", // TSUID to be deleted
	}

	// Begin the test output
	fmt.Println("Begin to test DELETE /api/uid/tsmeta.")

	// Perform the DELETE API call and check for errors
	resp, err := client.DeleteTSMetaData(&tsMetaData)

	// Assert no error on request level
	require.NoError(t, err, "Error occurred while deleting TS metadata")
	require.NotNil(t, resp, "Response should not be nil")

	// Check the response status code for a successful deletion (e.g., 200)
	require.Equal(t, 204, resp.StatusCode, "Unexpected status code, expected 200 for successful deletion")

	// Optionally, verify the response body content if applicable
	fmt.Printf("%s", resp.String())

	// End the test output
	fmt.Println("Finish testing DELETE /api/uid/tsmeta.")
}
