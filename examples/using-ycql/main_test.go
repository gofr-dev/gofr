//go:build !all

package main

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/request"
)

func Test_YCQL_IntegrationShop(t *testing.T) {
	// call  the main function
	go main()

	time.Sleep(time.Second * 5)

	testcases := []struct {
		desc       string
		method     string
		endpoint   string
		statusCode int
		body       []byte
	}{
		{"get with name", http.MethodGet, "shop?name=Vikash", http.StatusOK, nil},
		{"create by id 4", http.MethodPost, "shop", http.StatusCreated,
			[]byte(`{"id": 4, "name": "Puma", "location": "Belandur" , "state": "karnataka"}`)},
		{"create by id 7", http.MethodPost, "shop", http.StatusCreated,
			[]byte(`{"id": 7, "name": "Kalash", "location": "Jehanabad", "state": "Bihar"}`)},
		{"get at invalid endpoint", http.MethodGet, "unknown", http.StatusNotFound, nil},
		{"get shop by id at invalid endpoint", http.MethodGet, "shop/id", http.StatusNotFound, nil},
		{"update shop at invalid endpoint", http.MethodPut, "shop", http.StatusMethodNotAllowed, nil},
		{"delete shop", http.MethodDelete, "shop/4", http.StatusNoContent, nil},
	}
	for i, tc := range testcases {
		req, _ := request.NewMock(tc.method, "http://localhost:8085/"+tc.endpoint, bytes.NewBuffer(tc.body))

		cl := http.Client{}

		resp, err := cl.Do(req)
		if err != nil {
			t.Errorf("TEST[%v] Failed.\tHTTP request encountered Err: %v", i, err)

			continue // move to next test
		}

		if resp.StatusCode != tc.statusCode {
			t.Errorf("TEST[%v] Failed.\tExpected %v\tGot %v\n%s", i, tc.statusCode, resp.StatusCode, tc.desc)
		}

		_ = resp.Body.Close()
	}
}
