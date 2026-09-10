package gofr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/ai/mcp"
	"gofr.dev/pkg/gofr/testutil"
)

func testRouterTools(t *testing.T, exclude ...string) (*App, *routerTools) {
	t.Helper()
	testutil.NewServerConfigs(t)

	app := New()
	ex := make(map[string]bool)

	for _, e := range exclude {
		ex[e] = true
	}

	return app, &routerTools{app: app, cfg: &mcpConfig{exclude: ex}}
}

func toolNames(specs []ai.ToolSpec) []string {
	names := make([]string, len(specs))
	for i, s := range specs {
		names[i] = s.Name
	}

	return names
}

func TestRouterTools_List_ReadOnlyByDefault(t *testing.T) {
	app, rt := testRouterTools(t)
	app.GET("/items", func(*Context) (any, error) { return nil, nil })
	app.POST("/items", func(*Context) (any, error) { return nil, nil })

	names := toolNames(rt.List())
	assert.Contains(t, names, "get_items")
	assert.NotContains(t, names, "post_items", "write handlers are not exposed by default")
}

func TestRouterTools_List_Excludes(t *testing.T) {
	app, rt := testRouterTools(t, "/secret")
	app.GET("/secret", func(*Context) (any, error) { return nil, nil })
	app.GET("/public", func(*Context) (any, error) { return nil, nil })

	names := toolNames(rt.List())
	assert.NotContains(t, names, "get_secret")
	assert.Contains(t, names, "get_public")
}

func TestRouterTools_List_ReadOnlyAccess(t *testing.T) {
	app, rt := testRouterTools(t)
	app.GET("/items", func(*Context) (any, error) { return nil, nil })

	specs := rt.List()
	require.Len(t, specs, 1)
	assert.Equal(t, ai.ReadOnly, specs[0].Access)
}

func TestRouterTools_Call_DispatchesThroughRouter(t *testing.T) {
	app, rt := testRouterTools(t)
	app.GET("/ping", func(*Context) (any, error) { return "pong", nil })

	res, err := rt.Call(t.Context(), "get_ping", nil)
	require.NoError(t, err)

	body, err := res.JSON()
	require.NoError(t, err)
	assert.Contains(t, string(body), "pong")
}

func TestRouterTools_Call_PathParam(t *testing.T) {
	app, rt := testRouterTools(t)
	app.GET("/echo/{id}", func(c *Context) (any, error) { return c.PathParam("id"), nil })

	res, err := rt.Call(t.Context(), "get_echo_id", json.RawMessage(`{"id":"42"}`))
	require.NoError(t, err)

	body, _ := res.JSON()
	assert.Contains(t, string(body), "42")
}

func TestRouterTools_Call_QueryParam(t *testing.T) {
	app, rt := testRouterTools(t)
	app.GET("/search", func(c *Context) (any, error) { return c.Param("q"), nil })

	res, err := rt.Call(t.Context(), "get_search", json.RawMessage(`{"q":"gofr"}`))
	require.NoError(t, err)

	body, _ := res.JSON()
	assert.Contains(t, string(body), "gofr")
}

// An array argument to a read tool must arrive as repeated query values so ctx.Params returns each.
func TestRouterTools_Call_ArrayQueryParam(t *testing.T) {
	app, rt := testRouterTools(t)
	app.GET("/search", func(c *Context) (any, error) { return c.Params("tag"), nil })

	res, err := rt.Call(t.Context(), "get_search", json.RawMessage(`{"tag":["a","b","c"]}`))
	require.NoError(t, err)

	body, _ := res.JSON()
	assert.Contains(t, string(body), `["a","b","c"]`)
}

func TestRouterTools_Call_UnknownTool(t *testing.T) {
	_, rt := testRouterTools(t)

	_, err := rt.Call(t.Context(), "get_missing", nil)
	require.ErrorIs(t, err, ai.ErrToolNotFound)
}

func TestRouterTools_Call_PropagatesAuthHeader(t *testing.T) {
	app, rt := testRouterTools(t)

	var gotAuth string

	app.UseMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			next.ServeHTTP(w, r)
		})
	})
	app.GET("/secure", func(*Context) (any, error) { return "ok", nil })

	ctx := mcp.WithHeaders(t.Context(), http.Header{"Authorization": []string{"Bearer tok-123"}})

	_, err := rt.Call(ctx, "get_secure", nil)
	require.NoError(t, err)
	assert.Equal(t, "Bearer tok-123", gotAuth, "the caller's auth header reaches the dispatched handler")
}

// Gating must hold at Call time, not only in List — a guessed write-tool name must not dispatch,
// since write handlers are never exposed.
func TestRouterTools_Call_WriteToolAlwaysRejected(t *testing.T) {
	app, rt := testRouterTools(t)
	app.POST("/orders", func(*Context) (any, error) { return "made", nil })

	_, err := rt.Call(t.Context(), "post_orders", json.RawMessage(`{}`))
	require.ErrorIs(t, err, ai.ErrToolNotFound)
}

func TestRouterTools_Call_ExcludedRejected(t *testing.T) {
	app, rt := testRouterTools(t, "/secret")
	app.GET("/secret", func(*Context) (any, error) { return "shh", nil })

	_, err := rt.Call(t.Context(), "get_secret", nil)
	require.ErrorIs(t, err, ai.ErrToolNotFound)
}

func TestRouterTools_Call_WellKnownRejected(t *testing.T) {
	app, rt := testRouterTools(t)
	app.GET("/.well-known/secret", func(*Context) (any, error) { return "shh", nil })

	_, err := rt.Call(t.Context(), toolName(http.MethodGet, "/.well-known/secret"), nil)
	require.ErrorIs(t, err, ai.ErrToolNotFound)
}

// A path parameter that could break out of its segment is rejected, so an allowed tool cannot be
// steered into another route. Regression test for the MCP path-traversal fix.
func TestRouterTools_Call_PathTraversalRejected(t *testing.T) {
	app, rt := testRouterTools(t)
	app.GET("/public/{id}", func(c *Context) (any, error) { return c.PathParam("id"), nil })

	for _, bad := range []string{"../secret", "a/b", "..", ".", "a/../b", ""} {
		_, err := rt.Call(t.Context(), "get_public_id", json.RawMessage(fmt.Sprintf(`{"id":%q}`, bad)))
		require.ErrorIsf(t, err, errUnsafePathParam, "value %q must be rejected", bad)
	}

	// A normal segment value still resolves.
	res, err := rt.Call(t.Context(), "get_public_id", json.RawMessage(`{"id":"WIDGET-1"}`))
	require.NoError(t, err)

	body, _ := res.JSON()
	assert.Contains(t, string(body), "WIDGET-1")
}

// The framework-registered favicon route is never exposed as an agent tool.
func TestRouterTools_List_ExcludesFavicon(t *testing.T) {
	app, rt := testRouterTools(t)
	app.GET("/favicon.ico", func(*Context) (any, error) { return nil, nil })
	app.GET("/items", func(*Context) (any, error) { return nil, nil })

	names := toolNames(rt.List())
	assert.NotContains(t, names, "get_favicon.ico")
	assert.Contains(t, names, "get_items")
}

func TestRouterTools_Call_AuthRequiredWithoutHeaderFails(t *testing.T) {
	app, rt := testRouterTools(t)

	app.UseMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusUnauthorized)

				return
			}

			next.ServeHTTP(w, r)
		})
	})
	app.GET("/secure", func(*Context) (any, error) { return "ok", nil })

	_, err := rt.Call(t.Context(), "get_secure", nil) // no auth header in context
	require.ErrorIs(t, err, errToolStatus)
}

func TestFilteredTools_OnlyIntersects(t *testing.T) {
	app, rt := testRouterTools(t)
	app.GET("/a", func(*Context) (any, error) { return nil, nil })
	app.GET("/b", func(*Context) (any, error) { return nil, nil })

	// Chaining Only must narrow, never re-expose a name dropped by the first filter.
	narrowed := rt.Only("get_a").Only("get_b")
	assert.Empty(t, narrowed.List())
}

func TestRouterTools_Only(t *testing.T) {
	app, rt := testRouterTools(t)
	app.GET("/a", func(*Context) (any, error) { return nil, nil })
	app.GET("/b", func(*Context) (any, error) { return nil, nil })

	only := rt.Only("get_a")
	assert.Equal(t, []string{"get_a"}, toolNames(only.List()))

	_, err := only.Call(t.Context(), "get_a", nil) // whitelisted -> dispatches
	require.NoError(t, err)

	_, err = only.Call(t.Context(), "get_b", nil) // not whitelisted -> rejected
	require.ErrorIs(t, err, ai.ErrToolNotFound)
}

// EnableMCP may be called before routes are registered: tools are discovered lazily, so a route
// added afterwards is still exposed. This guards against a regression to eager route snapshotting.
func TestEnableMCP_BeforeRoutesStillExposesThem(t *testing.T) {
	testutil.NewServerConfigs(t)
	t.Setenv("MCP_PORT", "0")

	app := New()
	app.AddLLM(&testLLM{})
	app.EnableMCP() // called first, before any route exists

	app.GET("/late", func(*Context) (any, error) { return nil, nil })

	tools := app.container.LLM().Tools()
	assert.Contains(t, toolNames(tools.List()), "get_late",
		"a route registered after EnableMCP is still discovered")
}

func TestEnableMCP_WithExcludedRoutesOption(t *testing.T) {
	testutil.NewServerConfigs(t)
	t.Setenv("MCP_PORT", "0")

	app := New()
	app.AddLLM(&testLLM{})
	app.GET("/secret", func(*Context) (any, error) { return nil, nil })
	app.GET("/public", func(*Context) (any, error) { return nil, nil })
	app.EnableMCP(WithExcludedRoutes("/secret"))

	names := toolNames(app.container.LLM().Tools().List())
	assert.NotContains(t, names, "get_secret")
	assert.Contains(t, names, "get_public")
}

func TestEnableMCP_ExposesToolsViaLLM(t *testing.T) {
	testutil.NewServerConfigs(t)
	t.Setenv("MCP_PORT", "0") // disable the network server; in-process tools still work

	app := New()
	app.GET("/ping", func(*Context) (any, error) { return "pong", nil })
	app.AddLLM(&testLLM{})
	app.EnableMCP()

	assert.Nil(t, app.mcpServer, "MCP_PORT=0 disables the server")

	tools := app.container.LLM().Tools()
	assert.Contains(t, toolNames(tools.List()), "get_ping")
}

func TestEnableMCP_ServerConfigured(t *testing.T) {
	testutil.NewServerConfigs(t) // provides a free HTTP port; MCP_PORT defaults to 8200

	app := New()
	app.GET("/ping", func(*Context) (any, error) { return "pong", nil })
	app.EnableMCP()

	require.NotNil(t, app.mcpServer, "EnableMCP configures the server when the port is available")
	assert.Equal(t, defaultMCPPort, app.mcpServer.port)
}

func schemaFor(specs []ai.ToolSpec, name string) string {
	for _, s := range specs {
		if s.Name == name {
			return string(s.InputSchema)
		}
	}

	return ""
}

// A route's tool schema describes its path parameters. Only read-only handlers become tools, so
// there is no request body to describe.
func TestToolSchema_PathParamsOnly(t *testing.T) {
	app, rt := testRouterTools(t)
	app.GET("/orders/{id}", func(*Context) (any, error) { return nil, nil })

	schema := schemaFor(rt.List(), "get_orders_id")
	assert.Contains(t, schema, `"id"`)
	assert.Contains(t, schema, `"required":["id"]`)
	assert.NotContains(t, schema, `"item"`)
}

func TestHelpers(t *testing.T) {
	assert.Equal(t, "get_users_id", toolName(http.MethodGet, "/users/{id}"))
	assert.True(t, isReadOnlyMethod(http.MethodGet))
	assert.False(t, isReadOnlyMethod(http.MethodPost))
	assert.Equal(t, []string{"id"}, pathParams("/users/{id:[0-9]+}"))
	assert.Equal(t, "42", scalar(json.RawMessage(`"42"`)))
	assert.Equal(t, "true", scalar(json.RawMessage(`true`)))
}

func TestRouterTools_List_ExposesQuery(t *testing.T) {
	app, rt := testRouterTools(t)
	app.QUERY("/search", func(*Context) (any, error) { return nil, nil })
	app.POST("/write", func(*Context) (any, error) { return nil, nil })

	specs := rt.List()
	names := toolNames(specs)

	assert.Contains(t, names, "query_search", "QUERY handlers are exposed (safe + idempotent)")
	assert.NotContains(t, names, "post_write", "write handlers remain unexposed")

	// The QUERY tool must advertise a required "body" argument for the query payload.
	var querySpec ai.ToolSpec

	for _, s := range specs {
		if s.Name == "query_search" {
			querySpec = s
		}
	}

	require.NotEmpty(t, querySpec.Name)
	assert.Equal(t, ai.ReadOnly, querySpec.Access, "QUERY is safe, so it is ReadOnly")

	schema := map[string]any{}
	require.NoError(t, json.Unmarshal(querySpec.InputSchema, &schema))

	props, _ := schema["properties"].(map[string]any)
	assert.Contains(t, props, "body", "QUERY tool schema must include a body property")
	assert.Contains(t, schema["required"], "body", "body must be required")
}

func TestRouterTools_Call_QueryForwardsBody(t *testing.T) {
	app, rt := testRouterTools(t)
	app.QUERY("/search", func(c *Context) (any, error) {
		payload := map[string]any{}
		if err := c.Bind(&payload); err != nil {
			return nil, err
		}

		return payload, nil
	})

	res, err := rt.Call(t.Context(), "query_search", json.RawMessage(`{"body":{"filter":"title"}}`))
	require.NoError(t, err)

	body, err := res.JSON()
	require.NoError(t, err)
	assert.Contains(t, string(body), `"filter":"title"`, "the QUERY body must reach the handler")
}

func TestRouterTools_Call_QueryWithPathParamAndBody(t *testing.T) {
	app, rt := testRouterTools(t)
	app.QUERY("/index/{name}/search", func(c *Context) (any, error) {
		payload := map[string]any{}
		_ = c.Bind(&payload)
		payload["index"] = c.PathParam("name")

		return payload, nil
	})

	res, err := rt.Call(t.Context(), "query_index_name_search",
		json.RawMessage(`{"name":"books","body":{"q":"go"}}`))
	require.NoError(t, err)

	body, _ := res.JSON()
	assert.Contains(t, string(body), `"index":"books"`)
	assert.Contains(t, string(body), `"q":"go"`)
}
