package grpc

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"

	"github.com/stretchr/testify/assert"
)

func TestExample_Get(t *testing.T) {
	tests := []struct {
		desc string
		id   string
		resp *Response
		err  error
	}{
		{"get success", "1", &Response{FirstName: "First", SecondName: "Second"}, nil},
		{"get non existent entity", "2", nil, errors.EntityNotFound{Entity: "name", ID: "2"}},
	}

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "http://dummy?id="+tc.id, nil)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, nil)

		grcpHandler := New()
		resp, err := grcpHandler.Get(ctx, &ID{Id: tc.id})

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}
