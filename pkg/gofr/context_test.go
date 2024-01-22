package gofr

import (
	"bytes"
	"context"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

func Test_newContextSuccess(t *testing.T) {
	httpRequest, err := http.NewRequestWithContext(context.Background(),
		http.MethodPost, "/test", bytes.NewBuffer([]byte(`{"key":"value"}`)))
	if err != nil {
		t.Fatalf("unable to create request with context %v", err)
	}

	req := gofrHTTP.NewRequest(httpRequest)

	ctx := newContext(nil, req, container.NewContainer(config.NewEnvFile("")))

	body := map[string]string{}

	err = ctx.Bind(&body)

	assert.Equal(t, map[string]string{"key": "value"}, body)
	assert.Nil(t, err)
}
