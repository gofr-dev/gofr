//go:build !all

package main

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/request"
)

func Test_CQL_IntegrationPersons(t *testing.T) {
	// call  the main function
	go main()

	time.Sleep(5 * time.Second)

	tests := []struct {
		desc       string
		method     string
		endpoint   string
		statusCode int
		body       []byte
	}{
		{"get by name", http.MethodGet, "persons?name=Vikash", http.StatusOK, nil},
		{"create all fields ", http.MethodPost, "persons", http.StatusCreated, []byte(`{"id": 7, "name": "Kali", "age": 40, "State": "Goa"}`)},
		{"create few fields", http.MethodPost, "persons", http.StatusCreated, []byte(`{"id": 8, "name": "Kali"}`)},
		{"delete by id", http.MethodDelete, "persons/7", http.StatusNoContent, nil},
		{"get unknown route", http.MethodGet, "unknown", http.StatusNotFound, nil},
		{"get invalid route", http.MethodGet, "persons/id", http.StatusNotFound, nil},
		{"update without id", http.MethodPut, "persons", http.StatusMethodNotAllowed, nil},
	}
	for i, tc := range tests {
		req, _ := request.NewMock(tc.method, "http://localhost:9094/"+tc.endpoint, bytes.NewBuffer(tc.body))

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
