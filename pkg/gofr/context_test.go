package gofr

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	gofrHTTP "gofr.dev/pkg/gofr/http"
)

func TestTrace_ReturnsSpanObject(t *testing.T) {
	// Mock dependencies: Redis, SQL database, logging
	// Initialize the class object
	ctx := &Context{Context: context.Background()}

	// Invoke the method
	span := ctx.Trace("test")

	assert.NotNil(t, span)
}

func TestContext_Body_Response(t *testing.T) {
	type TestStruct struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	respBody := TestStruct{
		ID:   1,
		Name: "Bob",
	}

	reqBody := []byte(`{"id":1,"name":"Bob"}`)

	httpRequest, _ := http.NewRequestWithContext(context.Background(),
		http.MethodPost, "/test", bytes.NewReader(reqBody))

	req := gofrHTTP.NewRequest(httpRequest)

	ctx := Context{Context: context.Background(), Request: req}

	body := TestStruct{}

	err := ctx.Bind(&body)

	assert.Equal(t, respBody, body)
	assert.Nil(t, err)
}

func Test_newContext(t *testing.T) {
	httpRequest, _ := http.NewRequestWithContext(context.Background(),
		http.MethodPost, "/test", http.NoBody)
	req := gofrHTTP.NewRequest(httpRequest)

	ctx := newContext(nil, req, nil)

	assert.Equal(t, &Context{Context: req.Context(),
		Request:   req,
		Container: nil,
		responder: nil,
	}, ctx)
}
