package gofr

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

func TestSwaggerUIHandlerMissingAssetReturnsNotFound(t *testing.T) {
	testContainer, _ := container.NewMockContainer(t)

	fileName := "security.txt"
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.well-known/"+fileName, http.NoBody)
	req = mux.SetURLVars(req, map[string]string{"name": fileName})

	ctx := newContext(
		gofrHTTP.NewResponder(httptest.NewRecorder(), http.MethodGet),
		gofrHTTP.NewRequest(req), testContainer,
	)

	result, err := SwaggerUIHandler(ctx)
	require.Nil(t, result)

	var notFound gofrHTTP.ErrorEntityNotFound
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, "file", notFound.Name)
	assert.Equal(t, fileName, notFound.Value)
	assert.Equal(t, http.StatusNotFound, notFound.StatusCode())
}
