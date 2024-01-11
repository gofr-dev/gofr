package gofr

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logger"
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

	assert.Equal(t, respBody, body, "TEST, Failed.\n body binded to struct")
	assert.Nil(t, err, "TEST, Failed.\n body binded to struct")
}

func Test_newContext(t *testing.T) {
	httpRequest, _ := http.NewRequestWithContext(context.Background(),
		http.MethodPost, "/test", http.NoBody)
	req := gofrHTTP.NewRequest(httpRequest)

	ctx := newContext(nil, req, container.NewContainer(config.NewEnvFile("")))

	assert.Equal(t, &Context{Context: req.Context(),
		Request:   req,
		Container: &container.Container{Logger: logger.NewLogger(2)},
		responder: nil,
	}, ctx, "TEST, Failed.\n context creation successful")
}
