package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"

	"github.com/stretchr/testify/assert"
)

func TestValidateEntry(t *testing.T) {
	app := gofr.New()

	tests := []struct {
		desc string
		body details
		err  error
	}{
		{"create success", details{"+912123456789098", "c.r@yahoo.com"}, nil},
		{"create fail with missing phone",
			details{}, errors.InvalidParam{Param: []string{"Phone Number length"}}},
	}

	for i, tc := range tests {
		b, _ := json.Marshal(tc.body)
		body := bytes.NewReader(b)
		r := httptest.NewRequest(http.MethodPost, "http://localhost:9010/phone", body)
		req := request.NewHTTPRequest(r)

		ctx := gofr.NewContext(nil, req, app)

		_, err := ValidateEntry(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}
