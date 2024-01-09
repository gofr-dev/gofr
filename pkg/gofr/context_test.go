package gofr

import (
	"bytes"
	"context"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	gofrHTTP "gofr.dev/pkg/gofr/http"
)

func TestTrace_ReturnsSpanObject(t *testing.T) {
	ctx := &Context{Context: context.Background()}

	span := ctx.Trace("test")

	assert.NotNil(t, span, "TEST, Failed.\nspan creation successful")
}

func TestContext_Body_Response(t *testing.T) {
	type testStruct struct {
		ID   int    `json:"ID"`
		Name string `json:"Name"`
	}

	respBody := testStruct{
		ID:   1,
		Name: "Bob",
	}

	reqBody := []byte(`{"ID":1,"Name":"Bob"}`)

	httpRequest, _ := http.NewRequestWithContext(context.Background(),
		http.MethodPost, "/test", bytes.NewReader(reqBody))

	req := gofrHTTP.NewRequest(httpRequest)

	ctx := Context{Context: context.Background(), Request: req}

	body := testStruct{}

	err := ctx.Bind(&body)

	assert.Equalf(t, respBody, body, "TEST, Failed.\n body binded to struct")
	assert.Nilf(t, err, "TEST, Failed.\n body binded to struct")
}

func Test_newContext(t *testing.T) {
	httpRequest, _ := http.NewRequestWithContext(context.Background(),
		http.MethodPost, "/test", http.NoBody)
	req := gofrHTTP.NewRequest(httpRequest)

	ctx := newContext(nil, req, newContainer(config.NewEnvFile("")))

	assert.Equal(t, &Context{Context: req.Context(),
		Request:   req,
		Container: &Container{Logger: logging.NewLogger()},
		responder: nil,
	}, ctx, "TEST, Failed.\n context creation successful")
}
