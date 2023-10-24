package main

import (
	"bytes"
	"net/http"
	"os"
	"testing"
	"time"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/gofr"
)

const index = "customers"

// creating the index 'customers' and populating data to use it in tests
func TestMain(m *testing.M) {
	app := gofr.New()

	const mapping = `{"settings": {"number_of_shards": 1},"mappings": {"_doc": {"properties": {
				"id": {"type": "text"},"name": {"type": "text"},"city": {"type": "text"}}}}}`

	es := app.Elasticsearch
	_, err := es.Indices.Create(index,
		es.Indices.Create.WithBody(bytes.NewReader([]byte(mapping))),
		es.Indices.Create.WithPretty(),
	)

	if err != nil {
		app.Logger.Errorf("error creating index: %s", err.Error())
	}

	seeder := datastore.NewSeeder(&app.DataStore, "../../db")
	seeder.RefreshElasticSearch(app.Logger, index)

	os.Exit(m.Run())
}

func TestRoutes(t *testing.T) {
	go main()
	time.Sleep(time.Second * 2)

	tests := []struct {
		desc       string
		method     string
		endpoint   string
		statusCode int
		body       []byte
	}{
		{"get all customer success case", http.MethodGet, "customer", http.StatusOK, nil},
		{"get non existent customer", http.MethodGet, "customer/7", http.StatusInternalServerError, nil},
		{"create success", http.MethodPost, "customer", http.StatusCreated, []byte(`{"id":"100","name":"test","city":"xyz"}`)},
		{"update success", http.MethodPut, "customer/100", http.StatusOK, []byte(`{"id":"100","name":"test1","city":"xyz2"}`)},
		{"delete success", http.MethodDelete, "customer/100", http.StatusNoContent, nil},
	}

	for i, tc := range tests {
		req, _ := http.NewRequest(tc.method, "http://localhost:8001/"+tc.endpoint, bytes.NewBuffer(tc.body))
		c := http.Client{}

		resp, err := c.Do(req)
		if err != nil {
			t.Errorf("TEST[%v] Failed.\tHTTP request encountered Err: %v\n%s", i, err, tc.desc)
			continue
		}

		if resp.StatusCode != tc.statusCode {
			t.Errorf("TEST[%v] Failed.\tExpected %v\tGot %v\n%s", i, tc.statusCode, resp.StatusCode, tc.desc)
		}

		_ = resp.Body.Close()
	}
}
