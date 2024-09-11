package solr

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
)

func TestSolr(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
  		"responseHeader": {
    	"rf": 1,
    	"status": 0}}`))
	}))

	defer ts.Close()

	a := ts.Listener.Addr().String()
	addr := strings.Split(a, ":")

	ctrl := gomock.NewController(t)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug(gomock.Any()).Times(9)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_solr_stats", gomock.Any(), "type", gomock.Any()).Times(9)

	s := New(Config{Host: addr[0], Port: addr[1]})
	s.metrics = mockMetrics
	s.logger = mockLogger

	testClientSearch(t, s)
	testClientAddField(t, s)
	testClientCreate(t, s)
	testClientDelete(t, s)
	testClientDeleteField(t, s)
	testClientListFields(t, s)
	testClientRetrieve(t, s)
	testClientUpdate(t, s)
	testClientUpdateField(t, s)
}

func testClientSearch(t *testing.T, s *Client) {
	resp, err := s.Search(context.TODO(), "test", map[string]interface{}{"id": []string{"1234"}})
	if err != nil {
		t.Errorf("Expected error as nil but got %v", err)
	}

	if resp == nil {
		t.Errorf("Expected non nil response")
	}
}

func Test_InvalidRequest(t *testing.T) {
	_, err := call(context.TODO(), "GET", ":/localhost:", nil, nil)

	if err == nil {
		t.Errorf("Expected error due to invalid request, Got nil")
	}
}

func Test_InvalidJSONBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`Not a JSON`))
	}))
	defer ts.Close()

	_, err := call(context.TODO(), "GET", ts.URL, nil, nil)
	if err == nil {
		t.Errorf("Expected error due to invalid JSON body, Got nil")
	}
}

func testClientCreate(t *testing.T, s *Client) {
	body := bytes.NewBuffer([]byte(`{
		"id": "1234567",
		"cat": [
			"Book"
		],
		"genere_s": "Hello There"}`))

	resp, err := s.Create(context.TODO(), "test", body, map[string]interface{}{"commit": "true"})
	if err != nil {
		t.Errorf("Expected error as nil but got %v", err)
	}

	if resp == nil {
		t.Errorf("Expected non nil response")
	}
}

func testClientUpdate(t *testing.T, s *Client) {
	body := bytes.NewBuffer([]byte(`{
		"id": "1234567",
		"cat": [
			"Book"
		]}`))
	resp, err := s.Update(context.TODO(), "test", body, map[string]interface{}{"commit": "true"})

	if err != nil {
		t.Errorf("Expected error as nil but got %v", err)
	}

	if resp == nil {
		t.Errorf("Expected non nil response")
	}
}

func testClientDelete(t *testing.T, s *Client) {
	body := bytes.NewBuffer([]byte(`{"delete":[
		"1234",
		"12345"
	]}`))

	resp, err := s.Delete(context.TODO(), "test", body, map[string]interface{}{"commit": "true"})
	if err != nil {
		t.Errorf("Expected error as nil but got %v", err)
	}

	if resp == nil {
		t.Errorf("Expected non nil response")
	}
}

func Test_ErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "some error", http.StatusLocked)
	}))
	ts.Close()

	_, err := call(context.TODO(), "GET", ts.URL, nil, nil)
	if err == nil {
		t.Errorf("Expected error due to invalid JSON body, Got nil")
	}
}

func testClientRetrieve(t *testing.T, s *Client) {
	resp, err := s.Retrieve(context.TODO(), "test", map[string]interface{}{"wt": "xml"})
	if err != nil {
		t.Errorf("Expected error as nil but got %v", err)
	}

	if resp == nil {
		t.Errorf("Expected non nil response")
	}
}

func testClientListFields(t *testing.T, s *Client) {
	resp, err := s.ListFields(context.TODO(), "test", map[string]interface{}{"includeDynamic": true})

	if err != nil {
		t.Errorf("Expected error as nil but got %v", err)
	}

	if resp == nil {
		t.Errorf("Expected non nil response")
	}
}

func testClientAddField(t *testing.T, s *Client) {
	body := bytes.NewBuffer([]byte(`{"add-field":{
		"name":"merchant",
		"type":"string",
		"stored":true }}`))
	resp, err := s.AddField(context.TODO(), "test", body)

	if err != nil {
		t.Errorf("Expected error as nil but got %v", err)
	}

	if resp == nil {
		t.Errorf("Expected non nil response")
	}
}

func testClientUpdateField(t *testing.T, s *Client) {
	body := bytes.NewBuffer([]byte(`{"replace-field":{
		"name":"merchant",
		"type":"text_general"}}`))

	resp, err := s.UpdateField(context.TODO(), "test", body)
	if err != nil {
		t.Errorf("Expected error as nil but got %v", err)
	}

	if resp == nil {
		t.Errorf("Expected non nil response")
	}
}

func testClientDeleteField(t *testing.T, s *Client) {
	body := bytes.NewBuffer([]byte(`{"delete-field":{
		"name":"merchant",
		"type":"text_general"}}`))

	resp, err := s.DeleteField(context.TODO(), "test", body)
	if err != nil {
		t.Errorf("Expected error as nil but got %v", err)
	}

	if resp == nil {
		t.Errorf("Expected non nil response")
	}
}
