package gofr

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/ai/mcp"
	"gofr.dev/pkg/gofr/testutil"
)

func testRouterTools(t *testing.T, write bool, exclude ...string) (*App, *routerTools) {
	t.Helper()
	testutil.NewServerConfigs(t)

	app := New()
	ex := make(map[string]bool)

	for _, e := range exclude {
		ex[e] = true
	}

	return app, &routerTools{app: app, cfg: &mcpConfig{writeTools: write, exclude: ex}}
}

func toolNames(specs []ai.ToolSpec) []string {
	names := make([]string, len(specs))
	for i, s := range specs {
		names[i] = s.Name
	}

	return names
}

func TestRouterTools_List_ReadOnlyByDefault(t *testing.T) {
	app, rt := testRouterTools(t, false)
	app.GET("/items", func(*Context) (any, error) { return nil, nil })
	app.POST("/items", func(*Context) (any, error) { return nil, nil })

	names := toolNames(rt.List())
	assert.Contains(t, names, "get_items")
	assert.NotContains(t, names, "post_items", "write handlers are not exposed by default")
}

func TestRouterTools_List_WithWriteTools(t *testing.T) {
	app, rt := testRouterTools(t, true)
	app.GET("/items", func(*Context) (any, error) { return nil, nil })
	app.POST("/items", func(*Context) (any, error) { return nil, nil })

	names := toolNames(rt.List())
	assert.Contains(t, names, "get_items")
	assert.Contains(t, names, "post_items")
}

func TestRouterTools_List_Excludes(t *testing.T) {
	app, rt := testRouterTools(t, false, "/secret")
	app.GET("/secret", func(*Context) (any, error) { return nil, nil })
	app.GET("/public", func(*Context) (any, error) { return nil, nil })

	names := toolNames(rt.List())
	assert.NotContains(t, names, "get_secret")
	assert.Contains(t, names, "get_public")
}

func TestRouterTools_List_ReadOnlyAccess(t *testing.T) {
	app, rt := testRouterTools(t, true)
	app.GET("/items", func(*Context) (any, error) { return nil, nil })

	specs := rt.List()
	require.Len(t, specs, 1)
	assert.Equal(t, ai.ReadOnly, specs[0].Access)
}

func TestRouterTools_Call_DispatchesThroughRouter(t *testing.T) {
	app, rt := testRouterTools(t, false)
	app.GET("/ping", func(*Context) (any, error) { return "pong", nil })

	res, err := rt.Call(t.Context(), "get_ping", nil)
	require.NoError(t, err)

	body, err := res.JSON()
	require.NoError(t, err)
	assert.Contains(t, string(body), "pong")
}

func TestRouterTools_Call_PathParam(t *testing.T) {
	app, rt := testRouterTools(t, false)
	app.GET("/echo/{id}", func(c *Context) (any, error) { return c.PathParam("id"), nil })

	res, err := rt.Call(t.Context(), "get_echo_id", json.RawMessage(`{"id":"42"}`))
	require.NoError(t, err)

	body, _ := res.JSON()
	assert.Contains(t, string(body), "42")
}

func TestRouterTools_Call_QueryParam(t *testing.T) {
	app, rt := testRouterTools(t, false)
	app.GET("/search", func(c *Context) (any, error) { return c.Param("q"), nil })

	res, err := rt.Call(t.Context(), "get_search", json.RawMessage(`{"q":"gofr"}`))
	require.NoError(t, err)

	body, _ := res.JSON()
	assert.Contains(t, string(body), "gofr")
}

func TestRouterTools_Call_WriteBody(t *testing.T) {
	app, rt := testRouterTools(t, true)
	app.POST("/orders", func(c *Context) (any, error) {
		var in struct {
			Item string `json:"item"`
		}

		_ = c.Bind(&in)

		return in.Item, nil
	})

	res, err := rt.Call(t.Context(), "post_orders", json.RawMessage(`{"item":"book"}`))
	require.NoError(t, err)

	body, _ := res.JSON()
	assert.Contains(t, string(body), "book")
}

// A WithInput route dispatched as a tool must deliver the body args to ctx.Bind, end to end.
func TestRouterTools_Call_WithInputBodyReachesBind(t *testing.T) {
	app, rt := testRouterTools(t, true)
	app.POST("/orders", func(c *Context) (any, error) {
		var in orderInput

		require.NoError(t, c.Bind(&in))

		return map[string]any{"item": in.Item, "qty": in.Quantity}, nil
	}, WithInput[orderInput]())

	res, err := rt.Call(t.Context(), "post_orders", json.RawMessage(`{"item":"book","qty":3}`))
	require.NoError(t, err)

	body, _ := res.JSON()
	assert.Contains(t, string(body), "book")
	assert.Contains(t, string(body), "3")
}

// An array argument to a read tool must arrive as repeated query values so ctx.Params returns each.
func TestRouterTools_Call_ArrayQueryParam(t *testing.T) {
	app, rt := testRouterTools(t, false)
	app.GET("/search", func(c *Context) (any, error) { return c.Params("tag"), nil })

	res, err := rt.Call(t.Context(), "get_search", json.RawMessage(`{"tag":["a","b","c"]}`))
	require.NoError(t, err)

	body, _ := res.JSON()
	assert.Contains(t, string(body), `["a","b","c"]`)
}

func TestRouterTools_Call_UnknownTool(t *testing.T) {
	_, rt := testRouterTools(t, false)

	_, err := rt.Call(t.Context(), "get_missing", nil)
	require.ErrorIs(t, err, ai.ErrToolNotFound)
}

func TestRouterTools_Call_PropagatesAuthHeader(t *testing.T) {
	app, rt := testRouterTools(t, false)

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

// Gating must hold at Call time, not only in List — a guessed name must not reach a hidden tool.
func TestRouterTools_Call_WriteToolRejectedWhenReadOnly(t *testing.T) {
	app, rt := testRouterTools(t, false)
	app.POST("/orders", func(*Context) (any, error) { return "made", nil })

	_, err := rt.Call(t.Context(), "post_orders", json.RawMessage(`{}`))
	require.ErrorIs(t, err, ai.ErrToolNotFound)
}

func TestRouterTools_Call_ExcludedRejected(t *testing.T) {
	app, rt := testRouterTools(t, false, "/secret")
	app.GET("/secret", func(*Context) (any, error) { return "shh", nil })

	_, err := rt.Call(t.Context(), "get_secret", nil)
	require.ErrorIs(t, err, ai.ErrToolNotFound)
}

func TestRouterTools_Call_WellKnownRejected(t *testing.T) {
	app, rt := testRouterTools(t, false)
	app.GET("/.well-known/secret", func(*Context) (any, error) { return "shh", nil })

	_, err := rt.Call(t.Context(), toolName(http.MethodGet, "/.well-known/secret"), nil)
	require.ErrorIs(t, err, ai.ErrToolNotFound)
}

func TestRouterTools_Call_AuthRequiredWithoutHeaderFails(t *testing.T) {
	app, rt := testRouterTools(t, false)

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
	app, rt := testRouterTools(t, false)
	app.GET("/a", func(*Context) (any, error) { return nil, nil })
	app.GET("/b", func(*Context) (any, error) { return nil, nil })

	// Chaining Only must narrow, never re-expose a name dropped by the first filter.
	narrowed := rt.Only("get_a").Only("get_b")
	assert.Empty(t, narrowed.List())
}

func TestRouterTools_Only(t *testing.T) {
	app, rt := testRouterTools(t, false)
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

func TestEnableMCP_WithWriteToolsOption(t *testing.T) {
	testutil.NewServerConfigs(t)
	t.Setenv("MCP_PORT", "0")

	app := New()
	app.AddLLM(&testLLM{})
	app.POST("/orders", func(*Context) (any, error) { return nil, nil })
	app.EnableMCP(WithWriteTools())

	assert.Contains(t, toolNames(app.container.LLM().Tools().List()), "post_orders")
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

type richInput struct {
	Score float64  `json:"score"`
	Tags  []string `json:"tags"`
	Plain string   // no json tag -> field name is used
}

func TestWithInput_SchemaTypes(t *testing.T) {
	app, rt := testRouterTools(t, true)
	app.POST("/x", func(*Context) (any, error) { return nil, nil }, WithInput[richInput]())

	schema := schemaFor(rt.List(), "post_x")
	assert.Contains(t, schema, `"score":{"type":"number"}`)
	assert.Contains(t, schema, `"tags":{"type":"array"}`)
	assert.Contains(t, schema, `"Plain"`) // untagged field keeps its Go name
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

type orderInput struct {
	Item     string `json:"item"`
	Quantity int    `json:"qty"`
	Rush     bool   `json:"rush"`
	Internal string `json:"-"`
}

func schemaFor(specs []ai.ToolSpec, name string) string {
	for _, s := range specs {
		if s.Name == name {
			return string(s.InputSchema)
		}
	}

	return ""
}

func TestWithInput_EnrichesToolSchema(t *testing.T) {
	app, rt := testRouterTools(t, true)
	app.POST("/orders", func(*Context) (any, error) { return nil, nil }, WithInput[orderInput]())

	schema := schemaFor(rt.List(), "post_orders")
	require.NotEmpty(t, schema)
	assert.Contains(t, schema, `"item"`)
	assert.Contains(t, schema, `"qty"`)
	assert.Contains(t, schema, `"integer"`)   // qty
	assert.Contains(t, schema, `"boolean"`)   // rush
	assert.NotContains(t, schema, "Internal") // json:"-" field skipped
}

func TestWithInput_MergesPathParamsAndBody(t *testing.T) {
	app, rt := testRouterTools(t, true)
	app.PUT("/orders/{id}", func(*Context) (any, error) { return nil, nil }, WithInput[orderInput]())

	schema := schemaFor(rt.List(), "put_orders_id")
	require.NotEmpty(t, schema)
	assert.Contains(t, schema, `"id"`)   // path param
	assert.Contains(t, schema, `"item"`) // body field
	assert.Contains(t, schema, `"required":["id"]`)
}

type orderWithID struct {
	ID   int    `json:"id"`
	Item string `json:"item"`
}

// A body field whose name collides with a path parameter keeps the path-param schema (string),
// because at dispatch the value is used as the path segment, not the body.
func TestWithInput_PathParamWinsOverBodyField(t *testing.T) {
	app, rt := testRouterTools(t, true)
	app.PUT("/orders/{id}", func(*Context) (any, error) { return nil, nil }, WithInput[orderWithID]())

	schema := schemaFor(rt.List(), "put_orders_id")
	assert.Contains(t, schema, `"id":{"type":"string"}`)
	assert.NotContains(t, schema, `"id":{"type":"integer"}`)
	assert.Contains(t, schema, `"item"`)
}

func TestWithInput_AbsentIsPathParamsOnly(t *testing.T) {
	app, rt := testRouterTools(t, false)
	app.GET("/orders/{id}", func(*Context) (any, error) { return nil, nil })

	schema := schemaFor(rt.List(), "get_orders_id")
	assert.Contains(t, schema, `"id"`)
	assert.NotContains(t, schema, `"item"`)
}

func TestHelpers(t *testing.T) {
	assert.Equal(t, "get_users_id", toolName(http.MethodGet, "/users/{id}"))
	assert.Equal(t, ai.ReadOnly, accessForMethod(http.MethodGet))
	assert.Equal(t, ai.Write, accessForMethod(http.MethodPost))
	assert.Equal(t, []string{"id"}, pathParams("/users/{id:[0-9]+}"))
	assert.Equal(t, "42", scalar(json.RawMessage(`"42"`)))
	assert.Equal(t, "true", scalar(json.RawMessage(`true`)))
}
