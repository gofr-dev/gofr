package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	gogprc "google.golang.org/grpc"

	grpcServer "gofr.dev/examples/sample-grpc/handler/grpc"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/request"
)

func TestIntegration(t *testing.T) {
	go runGRPCServer(":10000")

	go main()

	time.Sleep(3 * time.Second)

	req, _ := request.NewMock(http.MethodGet, "http://localhost:9001/trace", http.NoBody)
	c := http.Client{}

	resp, err := c.Do(req)
	if err != nil {
		t.Errorf("TEST Failed.\tHTTP request encountered Err: %v\n", err)
		return
	}

	data := struct {
		Data string `json:"data"`
	}{}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("TEST Failed.\tUnable to read response body Err: %v\n", err)
		return
	}

	err = json.Unmarshal(respBody, &data)
	if err != nil {
		t.Errorf("TEST Failed.\tUnable to unmarshal response body Err: %v\n", err)
		return
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode, "TEST Failed: Success Case")
	assert.Equal(t, "ok", data.Data, "TEST Failed: Success Case")

	_ = resp.Body.Close()
}

func TestIntegration_Failure(t *testing.T) {
	go runGRPCServer(":10001")

	go main()

	time.Sleep(3 * time.Second)

	type response struct {
		Errors []errors.Response `json:"errors"`
	}

	testcases := []struct {
		desc      string
		endpoint  string
		method    string
		expStatus int
		expRes    string
	}{
		{"Invalid Method", "trace", http.MethodPost,
			http.StatusMethodNotAllowed, "POST method not allowed for Route POST /trace"},
		{"Invalid Endpoint", "trace2", http.MethodGet,
			http.StatusNotFound, "Route GET /trace2 not found"},
	}

	for i, tc := range testcases {
		req, _ := request.NewMock(tc.method, "http://localhost:9001/"+tc.endpoint, http.NoBody)
		c := http.Client{}

		resp, err := c.Do(req)
		if err != nil {
			t.Errorf("TEST Failed.\tHTTP request encountered Err: %v\n", err)
			return
		}

		var resErr response

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("TEST Failed.\tUnable to read response body Err: %v\n", err)
			return
		}

		err = json.Unmarshal(respBody, &resErr)
		if err != nil {
			t.Errorf("TEST Failed.\tUnable to unmarshal response body Err: %v\n", err)
			return
		}

		assert.Equal(t, tc.expStatus, resp.StatusCode, fmt.Sprintf("TEST[%d] Failed: %v", i+1, tc.desc))

		assert.Equal(t, tc.expRes, resErr.Errors[0].Error(), fmt.Sprintf("TEST[%d] Failed: %v", i+1, tc.desc))

		_ = resp.Body.Close()
	}
}

func runGRPCServer(port string) {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	server := gogprc.NewServer(
		gogprc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
		gogprc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
	)

	grpcServer.RegisterExampleServiceServer(server, testHandler{})

	err = server.Serve(lis)
	if err != nil {
		log.Fatalf("failed to server: %v", err)
	}
}

type testHandler struct {
	grpcServer.UnimplementedExampleServiceServer
}

func (h testHandler) Get(context.Context, *grpcServer.ID) (*grpcServer.Response, error) {
	return &grpcServer.Response{FirstName: "John"}, nil
}
