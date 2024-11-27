package solr

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_InvalidRequest(t *testing.T) {
	client := New(Config{})

	_, _, err := client.call(context.Background(), "GET", ":/localhost:", nil, nil)

	require.Error(t, err, "TEST Failed.\n")
}

func Test_InvalidJSONBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`Not a JSON`))
	}))
	defer ts.Close()

	client := New(Config{})

	_, _, err := client.call(context.Background(), "GET", ts.URL, nil, nil)

	require.Error(t, err, "TEST Failed.\n")
}

func Test_ErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "some error", http.StatusLocked)
	}))
	ts.Close()

	client := New(Config{})

	_, _, err := client.call(context.Background(), "GET", ts.URL, nil, nil)

	require.Error(t, err, "TEST Failed.\n")
}

func setupClient(t *testing.T) *Client {
	t.Helper()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
  		"responseHeader": {
    	"rf": 1,
    	"status": 0}}`))
	}))

	t.Cleanup(func() {
		ts.Close()
	})

	a := ts.Listener.Addr().String()
	addr := strings.Split(a, ":")

	ctrl := gomock.NewController(t)
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	mockLogger.EXPECT().Debug(gomock.Any())
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_solr_stats", gomock.Any(), "type", gomock.Any())

	s := New(Config{Host: addr[0], Port: addr[1]})
	s.metrics = mockMetrics
	s.logger = mockLogger

	return s
}

func Test_ClientSearch(t *testing.T) {
	s := setupClient(t)

	resp, err := s.Search(context.Background(), "test", map[string]any{"id": []string{"1234"}})

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientCreate(t *testing.T) {
	s := setupClient(t)

	body := bytes.NewBufferString(`{
		"id": "1234567",
		"cat": [
			"Book"
		],
		"genere_s": "Hello There"}`)

	resp, err := s.Create(context.Background(), "test", body, map[string]any{"commit": "true"})

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientUpdate(t *testing.T) {
	s := setupClient(t)

	body := bytes.NewBufferString(`{
		"id": "1234567",
		"cat": [
			"Book"
		]}`)
	resp, err := s.Update(context.Background(), "test", body, map[string]any{"commit": "true"})

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientDelete(t *testing.T) {
	s := setupClient(t)

	body := bytes.NewBufferString(`{"delete":[
		"1234",
		"12345"
	]}`)

	resp, err := s.Delete(context.Background(), "test", body, map[string]any{"commit": "true"})

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientRetrieve(t *testing.T) {
	s := setupClient(t)

	resp, err := s.Retrieve(context.Background(), "test", map[string]any{"wt": "xml"})

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientListFields(t *testing.T) {
	s := setupClient(t)

	resp, err := s.ListFields(context.Background(), "test", map[string]any{"includeDynamic": true})

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientAddField(t *testing.T) {
	s := setupClient(t)

	body := bytes.NewBufferString(`{"add-field":{
		"name":"merchant",
		"type":"string",
		"stored":true }}`)
	resp, err := s.AddField(context.Background(), "test", body)

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientUpdateField(t *testing.T) {
	s := setupClient(t)

	body := bytes.NewBufferString(`{"replace-field":{
		"name":"merchant",
		"type":"text_general"}}`)

	resp, err := s.UpdateField(context.Background(), "test", body)

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}

func Test_ClientDeleteField(t *testing.T) {
	s := setupClient(t)

	body := bytes.NewBufferString(`{"delete-field":{
		"name":"merchant",
		"type":"text_general"}}`)

	resp, err := s.DeleteField(context.Background(), "test", body)

	require.NoError(t, err, "TEST Failed.\n")
	require.NotNil(t, resp, "TEST Failed.\n")
}
