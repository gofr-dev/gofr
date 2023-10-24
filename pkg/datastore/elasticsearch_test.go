package datastore

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

func newClient(t *testing.T) *Elasticsearch {
	es, err := NewElasticsearchClient(log.NewMockLogger(io.Discard), &ElasticSearchCfg{Host: "localhost", Ports: []int{2012}})
	if err != nil {
		t.Errorf("error in creating elastic search client")

		return nil
	}

	_, err = es.Indices.Delete([]string{"test"}, es.Indices.Delete.WithIgnoreUnavailable(true))
	if err != nil {
		t.Errorf("error while deleting index")
	}

	return &es
}

func TestNewElasticsearchClient(t *testing.T) {
	testcases := []struct {
		description string
		config      ElasticSearchCfg
		errStr      string
	}{
		{"success", ElasticSearchCfg{Host: "localhost", Ports: []int{2012}}, ""},
		{"failure", ElasticSearchCfg{Host: "localhost", Ports: []int{2012}, CloudID: "data-id"},
			"cannot create client: both Addresses and CloudID are set"},
	}

	for i, tc := range testcases {
		_, err := NewElasticsearchClient(log.NewMockLogger(io.Discard), &tc.config)
		if err == nil {
			if tc.errStr != "" {
				t.Errorf("TESTCASE[%v] expected error string %v", i, tc.errStr)
			}
		} else {
			assert.Contains(t, err.Error(), tc.errStr, "TESTCASE[%v]", i)
		}
	}
}

func TestDataStore_ElasticsearchHealthCheck_ErrorLog(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	config := ElasticSearchCfg{CloudID: "test"}
	expectedResp := types.Health{Name: "elasticsearch", Status: pkg.StatusDown}
	expectedLogMessage := "Health check failed for elasticsearch Reason: Elastic search not initialized."
	conn, _ := NewElasticsearchClient(logger, &config)

	output := conn.HealthCheck()

	if !reflect.DeepEqual(output, expectedResp) {
		t.Errorf("[TESTCASE] Failed.\nExpected: %v,\nGot: %v", expectedResp, output)
	}

	if !assert.Contains(t, b.String(), expectedLogMessage) {
		t.Errorf("expected: %v, got: %v", expectedLogMessage, b.String())
	}
}

func TestDataStore_ElasticsearchHealthCheck(t *testing.T) {
	testCases := []struct {
		config   ElasticSearchCfg
		expected types.Health
	}{
		{ElasticSearchCfg{Host: "localhost", Ports: []int{2012}},
			types.Health{Name: "elasticsearch", Status: pkg.StatusUp, Host: "localhost"}},
		{ElasticSearchCfg{Host: "localhost", Ports: []int{2012}, CloudID: "data-cloud-id"},
			types.Health{Name: "elasticsearch", Status: pkg.StatusDown, Host: "localhost"}},
	}

	for i, tc := range testCases {
		conn, _ := NewElasticsearchClient(log.NewMockLogger(io.Discard), &tc.config)

		output := conn.HealthCheck()
		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("[TESTCASE%v] Failed.\nExpected: %v,\nGot: %v", i+1, tc.expected, output)
		}
	}
}

func Test_Ping(t *testing.T) {
	conn, _ := NewElasticsearchClient(log.NewMockLogger(io.Discard), &ElasticSearchCfg{Host: "localhost", Ports: []int{2012}})

	_, err := conn.Ping()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

type data struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func Test_bind(t *testing.T) {
	tcs := []struct {
		data interface{}
		resp interface{}
		err  error
	}{
		{data{Name: "test"}, make(map[string]string), nil},
		{make(chan int), make(map[string]string), &json.UnsupportedTypeError{}},
	}

	for i, tc := range tcs {
		err := bind(tc.data, &tc.resp)

		if tc.err == nil {
			assert.Equal(t, tc.err, err, "TESTCASE[%v]", i)
		} else if _, ok := err.(*json.UnsupportedTypeError); !ok {
			t.Errorf("TESTCASE[%v] expected error %v, got %v", i, tc.err, err)
		}
	}
}

func Test_bind_UnMarshalError(t *testing.T) {
	expErr := &json.UnmarshalTypeError{}

	err := bind(map[string]*data{"name": {Name: "test"}}, &data{})
	if err == nil {
		t.Errorf("expected error %v", expErr)
	}

	if _, ok := err.(*json.UnmarshalTypeError); !ok {
		t.Errorf("expected error of type %T", expErr)
	}
}

func Test_Bind(t *testing.T) {
	es := newClient(t)

	tcs := []struct {
		insertData bool
		expOut     data
	}{
		{false, data{ID: "", Name: ""}},
		{true, data{ID: "1", Name: "test"}},
	}

	for i, tc := range tcs {
		if tc.insertData {
			insertData(t, es, data{ID: "1", Name: "test"})
		}

		res, err := es.Search(
			es.Search.WithIndex("test"),
			es.Search.WithContext(context.Background()),
			es.Search.WithBody(strings.NewReader(`{"query" : { "match" : {"id":"1"} }}`)),
			es.Search.WithPretty(),
			es.Search.WithSize(1),
		)
		if err != nil {
			t.Errorf("TESTCASE[%v] error in making search request", i)
		}

		var d data

		err = es.Bind(res, &d)
		if err != nil {
			t.Errorf("TESTCASE[%v] expected no error, got %v", i, err)
		}

		if !reflect.DeepEqual(tc.expOut, d) {
			t.Errorf("TESTCASE[%v] expected %v, got %v", i, tc.expOut, d)
		}
	}
}

func Test_getDataError(t *testing.T) {
	es := newClient(t)
	insertData(t, es, data{ID: "1", Name: "test"})

	res, err := es.Search(
		es.Search.WithIndex("test"),
		es.Search.WithContext(context.Background()),
		es.Search.WithBody(strings.NewReader("")),
		es.Search.WithPretty(),
	)
	if err != nil {
		t.Errorf("error in making search request")
	}

	d := map[string]*data{}

	err = es.Bind(res, &d)
	if err == nil {
		t.Errorf("expected error %v, got none", json.UnmarshalTypeError{})
	}

	err = es.BindArray(res, &d)
	if err == nil {
		t.Errorf("expected error %v, got none", json.UnmarshalTypeError{})
	}
}

func Test_bindArrayError(t *testing.T) {
	es := newClient(t)
	insertData(t, es, data{ID: "1", Name: "test"})
	insertData(t, es, data{ID: "2", Name: "test1"})

	res, err := es.Search(
		es.Search.WithIndex("test"),
		es.Search.WithContext(context.Background()),
		es.Search.WithBody(strings.NewReader("")),
		es.Search.WithPretty(),
	)
	if err != nil {
		t.Errorf("error in making search request")
	}

	var dd [][]data

	err = es.BindArray(res, &dd)
	if err == nil {
		t.Errorf("expected error %v, got none", json.UnmarshalTypeError{})
	}
}

func Test_BindArray(t *testing.T) {
	es := newClient(t)
	insertData(t, es, data{ID: "1", Name: "test"})
	insertData(t, es, data{ID: "2", Name: "test2"})

	res, err := es.Search(
		es.Search.WithIndex("test"),
		es.Search.WithContext(context.Background()),
		es.Search.WithBody(strings.NewReader("")),
		es.Search.WithPretty(),
	)
	if err != nil {
		t.Errorf("error in making search request")
	}

	var d []data

	err = es.BindArray(res, &d)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	expData := []data{
		{ID: "2", Name: "test2"},
		{ID: "1", Name: "test"},
	}

	if !reflect.DeepEqual(expData, d) {
		t.Errorf("expected %v, got %v", expData, d)
	}
}

func insertData(t *testing.T, es *Elasticsearch, data data) {
	body, err := json.Marshal(data)
	if err != nil {
		t.Errorf("error in `marshaling` data while inserting data")
	}

	_, err = es.Index(
		"test",
		bytes.NewReader(body),
		es.Index.WithRefresh("true"),
		es.Index.WithPretty(),
		es.Index.WithContext(context.Background()),
		es.Index.WithDocumentID(data.ID),
	)
	if err != nil {
		t.Errorf("error in creating document")
	}
}
