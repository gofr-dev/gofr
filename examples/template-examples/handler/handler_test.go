package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

func TestTemplateHandler(t *testing.T) {
	app := gofr.New()
	dir := t.TempDir()
	app.TemplateDir = dir + "/../templates"
	r := httptest.NewRequest(http.MethodGet, "http://dummy/test", nil)
	req := request.NewHTTPRequest(r)

	ctx := gofr.NewContext(nil, req, app)
	if _, err := Template(ctx); err != nil {
		t.Errorf("FAILED, got error: %v", err)
	}
}
