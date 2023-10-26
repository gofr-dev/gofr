package main

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

func TestRoutes(t *testing.T) {
	t.Skip()

	go main()

	time.Sleep(time.Second * 5)

	testcases := []struct {
		desc       string
		method     string
		endpoint   string
		statusCode int
		body       []byte
	}{
		{"call get at unknown endpoint", http.MethodGet, "unknown", http.StatusNotFound, nil},
		{"call get at invalid endpoint", http.MethodGet, "/customer/id", http.StatusNotFound, nil},
		{"call get successfully", http.MethodGet, "customer?id=2", http.StatusOK, nil},
	}

	for i, tc := range testcases {
		req, _ := request.NewMock(tc.method, "http://localhost:8009/"+tc.endpoint, bytes.NewBuffer(tc.body))
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

func TestMain(m *testing.M) {
	app := gofr.New()

	host := os.Getenv("SOLR_HOST")
	port := os.Getenv("SOLR_PORT")

	resp, err := http.Get("http://localhost:" + port + "/solr/admin/collections?action=CREATE&name=customer&numShards=1&replicationFactor=1")
	if err != nil {
		app.Logger.Errorf("error in sending request")
		os.Exit(1)
	}

	_ = resp.Body.Close()

	client := datastore.NewSolrClient(host, port)
	body := []byte(`{
	"add-field": {
		"name": "id",
       "type": "int",
        "stored": "false",
	}}`)

	document := bytes.NewBuffer(body)
	_, _ = client.AddField(context.TODO(), "customer", document)

	body = []byte(`{
		"add-field": {
			"name": "name",
				"type": "string",
				"stored": "true"
		}
	}`)

	document = bytes.NewBuffer(body)
	_, _ = client.AddField(context.TODO(), "customer", document)

	body = []byte(`{
		"add-field":{
		   "name":"dateOfBirth",
		   "type":"string",
		"stored":true }}`)

	document = bytes.NewBuffer(body)
	_, _ = client.UpdateField(context.TODO(), "customer", document)

	body = []byte(`{
		     "add-field":{
			   "name":"name",
			   "type":"string",
		    "stored":true }
			}`)
	document = bytes.NewBuffer(body)
	_, _ = client.AddField(context.TODO(), "customer", document)

	os.Exit(m.Run())
}
