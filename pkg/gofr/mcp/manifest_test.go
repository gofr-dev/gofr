package mcp

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func newTestRouter(t *testing.T) *mux.Router {
	t.Helper()
	r := mux.NewRouter()
	r.NewRoute().Methods(http.MethodGet).Path("/users").Handler(http.NotFoundHandler())
	r.NewRoute().Methods(http.MethodGet).Path("/users/{id}").Handler(http.NotFoundHandler())
	r.NewRoute().Methods(http.MethodPost).Path("/users").Handler(http.NotFoundHandler())
	r.NewRoute().Methods(http.MethodDelete).Path("/users/{id}").Handler(http.NotFoundHandler())
	r.NewRoute().Methods(http.MethodGet).Path("/.well-known/health").Handler(http.NotFoundHandler())
	r.NewRoute().Methods(http.MethodGet).Path("/swagger").Handler(http.NotFoundHandler())
	return r
}

func TestBuildManifest_GETOnlyByDefault(t *testing.T) {
	m := BuildManifest(newTestRouter(t), nil, BuildOptions{})

	names := toolNames(m.Tools())
	assert.Contains(t, names, "get_users")
	assert.Contains(t, names, "get_users_id")
	assert.NotContains(t, names, "post_users", "POST must not be exposed by default")
	assert.NotContains(t, names, "delete_users_id")
}

func TestBuildManifest_AllowMutations(t *testing.T) {
	m := BuildManifest(newTestRouter(t), nil, BuildOptions{AllowMutations: true})

	names := toolNames(m.Tools())
	assert.Contains(t, names, "post_users")
	assert.Contains(t, names, "delete_users_id")
}

func TestBuildManifest_FiltersBuiltins(t *testing.T) {
	m := BuildManifest(newTestRouter(t), nil, BuildOptions{AllowMutations: true})

	for _, tool := range m.Tools() {
		assert.NotContains(t, tool.Name, "well_known")
		assert.NotContains(t, tool.Name, "swagger")
	}
}

func TestBuildManifest_LearnedSchemaSharpensInputs(t *testing.T) {
	type body struct {
		Name string `json:"name"`
	}

	l := NewLearner("")
	l.Register("POST", "/users", nil)
	l.RecordBind("POST", "/users", reflect.TypeFor[body]())

	m := BuildManifest(newTestRouter(t), l, BuildOptions{AllowMutations: true})

	var post Tool

	for _, tool := range m.Tools() {
		if tool.Name == "post_users" {
			post = tool
		}
	}

	props, _ := post.InputSchema["properties"].(map[string]any)
	bodySchema, _ := props["body"].(Schema)
	assert.Equal(t, "object", bodySchema["type"])

	bodyProps, _ := bodySchema["properties"].(map[string]any)
	assert.Contains(t, bodyProps, "name")
}

func TestBuildManifest_PathParamsRequired(t *testing.T) {
	m := BuildManifest(newTestRouter(t), nil, BuildOptions{})

	var byID Tool

	for _, tool := range m.Tools() {
		if tool.Name == "get_users_id" {
			byID = tool
		}
	}

	required, _ := byID.InputSchema["required"].([]string)
	assert.Contains(t, required, "id")
}

func TestBuildManifest_FindByName(t *testing.T) {
	m := BuildManifest(newTestRouter(t), nil, BuildOptions{})

	method, path, ok := m.Find("get_users_id")
	assert.True(t, ok)
	assert.Equal(t, http.MethodGet, method)
	assert.Equal(t, "/users/{id}", path)
}

func TestBuildManifest_FindUnknown(t *testing.T) {
	m := BuildManifest(newTestRouter(t), nil, BuildOptions{})

	_, _, ok := m.Find("get_nope")
	assert.False(t, ok)
}

func TestBuildManifest_NilRouter(t *testing.T) {
	m := BuildManifest(nil, nil, BuildOptions{})
	assert.Empty(t, m.Tools())
}

func toolNames(tools []Tool) []string {
	out := make([]string, 0, len(tools))
	for _, t := range tools {
		out = append(out, t.Name)
	}
	return out
}

func TestToolName_StripsRegexConstraints(t *testing.T) {
	assert.Equal(t, "get_users_id", toolName("GET", "/users/{id:[0-9]+}"))
	assert.Equal(t, "post_users", toolName("POST", "/users"))
	assert.Equal(t, "get_root", toolName("GET", "/"))
	assert.Equal(t, "get_a_b_c", toolName("GET", "/a/b/c"))
}

func TestExtractPathParams(t *testing.T) {
	assert.Equal(t, []string{"id"}, extractPathParams("/users/{id}"))
	assert.Equal(t, []string{"orderID", "itemID"}, extractPathParams("/orders/{orderID}/items/{itemID}"))
	assert.Equal(t, []string{"id"}, extractPathParams("/users/{id:[0-9]+}"))
	assert.Empty(t, extractPathParams("/users"))
}
