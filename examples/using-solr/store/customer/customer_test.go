package customer

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	
	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/using-solr/store"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

const er = "error"

type test struct {
	collection string
	wantErr    bool
}

func TestCustomer_ListError(t *testing.T) {
	collections := []string{"error", "json error"}
	c := New(mockSolrClient{})
	app := gofr.New()
	ctx := gofr.NewContext(nil, nil, app)

	for _, collection := range collections {
		_, err := c.List(ctx, collection, store.Filter{})
		if err == nil {
			t.Error("Expected error got nil")
		}
	}
}

func TestCustomer_ListResponse(t *testing.T) {
	c := New(mockSolrClient{})
	app := gofr.New()
	ctx := gofr.NewContext(nil, nil, app)
	expectedResp := []store.Model{{ID: 553573403, Name: "book", DateOfBirth: "01-01-1987"}}

	resp, err := c.List(ctx, "customer", store.Filter{})
	if err != nil {
		t.Errorf("Expected nil error\tGot %v", err)
	}

	assert.Equal(t, expectedResp, resp, "TEST Failed.\n")
}

func TestCustomer_Create(t *testing.T) {
	var testcases = []test{
		{"error", true},
		{"customer", false},
	}

	c := New(mockSolrClient{})
	app := gofr.New()
	ctx := gofr.NewContext(nil, nil, app)

	for _, tc := range testcases {
		err := c.Create(ctx, tc.collection, store.Model{})

		if (err == nil && tc.wantErr) || (err != nil && tc.wantErr == false) {
			t.Errorf("Expected %v\tGot %v\n", tc.wantErr, err)
		}
	}
}

func TestCustomer_Update(t *testing.T) {
	var testcases = []test{
		{"error", true},
		{"customer", false},
	}

	c := New(mockSolrClient{})
	app := gofr.New()
	ctx := gofr.NewContext(nil, nil, app)

	for _, tc := range testcases {
		err := c.Update(ctx, tc.collection, store.Model{})
		if (err == nil && tc.wantErr) || (err != nil && tc.wantErr == false) {
			t.Errorf("Expected %v\tGot %v\n", tc.wantErr, err)
		}
	}
}

func TestCustomer_Delete(t *testing.T) {
	var testcases = []test{
		{"error", true},
		{"customer", false},
	}

	c := New(mockSolrClient{})
	app := gofr.New()
	ctx := gofr.NewContext(nil, nil, app)

	for _, tc := range testcases {
		err := c.Delete(ctx, tc.collection, store.Model{})
		if (err == nil && tc.wantErr) || (err != nil && tc.wantErr == false) {
			t.Errorf("Expected %v\tGot %v\n", tc.wantErr, err)
		}
	}
}

type mockSolrClient struct{}

func (m mockSolrClient) Search(_ context.Context, collection string, _ map[string]interface{}) (interface{}, error) {
	if collection == er {
		return nil, errors.InvalidParam{}
	} else if collection == "json error" {
		b := []byte(`{"response": {
		"numFound": 1,
		"start": 0,
		"docs": [
			{	"id": "0553573403",
				"name": [
					"book"]}]}}`)
		var resp interface{}

		_ = json.Unmarshal(b, &resp)

		return datastore.Response{Code: http.StatusOK, Data: resp}, nil
	}

	b := []byte(`{"response": {
		"numFound": 1,
		"start": 0,
		"docs": [
			{	"id": "553573403",
				"name":"book",
                "dateOfBirth":"01-01-1987"}]}}`)

	var resp interface{}
	_ = json.Unmarshal(b, &resp)

	return datastore.Response{Code: http.StatusOK, Data: resp}, nil
}

func (m mockSolrClient) Create(_ context.Context, collection string, _ *bytes.Buffer, _ map[string]interface{}) (interface{}, error) {
	if collection == er {
		return nil, errors.InvalidParam{}
	}

	b := []byte(`{"responseHeader": {
		"rf": 1,
    	"status": 0`)

	var resp interface{}

	_ = json.Unmarshal(b, &resp)

	return datastore.Response{Code: http.StatusOK, Data: resp}, nil
}

func (m mockSolrClient) Update(_ context.Context, collection string, _ *bytes.Buffer, _ map[string]interface{}) (interface{}, error) {
	if collection == "error" {
		return nil, errors.InvalidParam{}
	}

	b := []byte(`{"responseHeader": {
		"rf": 1,
    	"status": 0`)

	var resp interface{}
	_ = json.Unmarshal(b, &resp)

	return datastore.Response{Code: http.StatusOK, Data: resp}, nil
}

func (m mockSolrClient) Delete(_ context.Context, collection string, _ *bytes.Buffer, _ map[string]interface{}) (interface{}, error) {
	if collection == "error" {
		return nil, errors.InvalidParam{}
	}

	b := []byte(`{"responseHeader": {
		"rf": 1,
    	"status": 0`)

	var resp interface{}
	_ = json.Unmarshal(b, &resp)

	return datastore.Response{Code: http.StatusOK, Data: resp}, nil
}
