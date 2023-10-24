package main

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"google.golang.org/grpc/credentials/insecure"

	grpc2 "gofr.dev/examples/sample-grpc/handler/grpc"
	"gofr.dev/pkg/gofr/request"

	"google.golang.org/grpc"
)

func TestIntegration(t *testing.T) {
	go main()
	time.Sleep(time.Second * 5)

	tests := []struct {
		desc       string
		method     string
		endpoint   string
		statusCode int
		body       []byte
	}{
		{"get success", http.MethodGet, "/example?id=1", http.StatusOK, nil},
		{"get non existent entity", http.MethodGet, "/example?id=2", http.StatusNotFound, nil},
		{"unregistered update route", http.MethodPut, "/example", http.StatusNotFound, []byte(`{}`)},
	}

	for i, tc := range tests {
		req, _ := request.NewMock(tc.method, "http://localhost:9093/"+tc.endpoint, bytes.NewBuffer(tc.body))
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

	testClient(t)
}

func testClient(tb testing.TB) {
	conn, err := grpc.Dial("localhost:10000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		tb.Errorf("did not connect: %s", err)
		return
	}

	defer conn.Close()

	c := grpc2.NewExampleServiceClient(conn)

	_, err = c.Get(context.TODO(), &grpc2.ID{Id: "1"})
	if err != nil {
		tb.Errorf("FAILED, Expected: %v, Got: %v", nil, err)
	}

	_, err = c.Get(context.TODO(), &grpc2.ID{Id: "2"})
	if err == nil {
		tb.Errorf("FAILED, Expected: %v, Got: %v", nil, err)
	}
}
