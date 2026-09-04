package gofr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/ai/mcp"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
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
	testutil.NewServerConfigs(t) // provides a free HTTP port; MCP_PORT is left unset

	app := New()
	app.GET("/ping", func(*Context) (any, error) { return "pong", nil })
	app.EnableMCP()

	// The assertion pins the default port, so this test cannot be handed a free one. That is only
	// safe because EnableMCP does no network I/O: whether 8200 happens to be occupied on the machine
	// running the suite is now irrelevant to the outcome.
	require.NotNil(t, app.mcpServer, "EnableMCP configures the server from config alone")
	assert.Equal(t, defaultMCPPort, app.mcpServer.port)
}

// TestEnableMCP_OccupiedPortStillConfigures is the regression test for the process exit.
//
// EnableMCP used to probe the port and call Logger.Fatalf when something held it, and that Fatalf
// calls os.Exit(1) — from library code, during setup. Any service whose MCP port was taken died at
// startup instead of serving, and in the test suite it killed the test binary mid-run, reporting
// every remaining test in the package as failed with nothing to identify the culprit.
func TestEnableMCP_OccupiedPortStillConfigures(t *testing.T) {
	testutil.NewServerConfigs(t)

	// A real listener, so the port is genuinely unavailable rather than assumed to be.
	occupied, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { _ = occupied.Close() })

	port := occupied.Addr().(*net.TCPAddr).Port
	t.Setenv("MCP_PORT", strconv.Itoa(port))

	app := New()
	app.GET("/ping", func(*Context) (any, error) { return "pong", nil })

	// Reaching the next line at all is the assertion: before the fix, this call exited the process.
	app.EnableMCP()

	require.NotNil(t, app.mcpServer, "an occupied port must not prevent configuration")
	assert.Equal(t, port, app.mcpServer.port)
}

// TestBindMCPServer_OccupiedPortAbortsStartup covers the other half of the policy: EnableMCP was
// called, so MCP was asked for, and a port that cannot be claimed has to stop the run rather than
// leave the service up without the transport it was configured to expose.
//
// The abort is a false return, not an os.Exit, so Run unwinds like a failed OnStart hook.
func TestBindMCPServer_OccupiedPortAbortsStartup(t *testing.T) {
	testutil.NewServerConfigs(t)

	occupied, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { _ = occupied.Close() })

	port := occupied.Addr().(*net.TCPAddr).Port
	t.Setenv("MCP_PORT", strconv.Itoa(port))

	var proceed bool

	// ERROR goes to the logger's errorOut, which is os.Stderr — and the logger captures it at
	// construction, so the app has to be built inside the capture.
	logs := testutil.StderrOutputForFunc(func() {
		app := New()
		app.GET("/ping", func(*Context) (any, error) { return "pong", nil })
		app.EnableMCP()

		proceed = app.bindMCPServer(t.Context())
	})

	assert.False(t, proceed, "startup must not continue when the MCP port cannot be claimed")
	assert.Contains(t, logs, "MCP server cannot start on port")
	// Deliberately not asserting the OS error text: Windows words EADDRINUSE differently
	// ("Only one usage of each socket address..."), and the framework's own remedy is the part of
	// the message this test is actually about.
	assert.Contains(t, logs, fmt.Sprintf("port %d", port))
	// The message has to be actionable: a bare error leaves the operator guessing at the remedy.
	assert.Contains(t, logs, "MCP_PORT=0")
}

// TestBindMCPServer_FreePortProceeds is the counterpart, and also pins that bind actually claims the
// port rather than reporting on it: the address must be occupied afterwards.
func TestBindMCPServer_FreePortProceeds(t *testing.T) {
	testutil.NewServerConfigs(t)
	t.Setenv("MCP_PORT", strconv.Itoa(testutil.GetFreePort(t)))

	app := New()
	app.GET("/ping", func(*Context) (any, error) { return "pong", nil })
	app.EnableMCP()

	require.True(t, app.bindMCPServer(t.Context()))
	require.NotNil(t, app.mcpServer.listener)

	t.Cleanup(func() { _ = app.mcpServer.listener.Close() })

	// A second bind of the same port must now fail, which is only true if the first one took it.
	_, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp",
		fmt.Sprintf("127.0.0.1:%d", app.mcpServer.port))
	require.Error(t, err, "bind must claim the port, not merely check it")
}

// TestBindMCPServer_NoServerIsNotAFailure — MCP_PORT=0, or EnableMCP never called, leaves no server
// to bind, and that must not be mistaken for a startup failure.
func TestBindMCPServer_NoServerIsNotAFailure(t *testing.T) {
	testutil.NewServerConfigs(t)
	t.Setenv("MCP_PORT", "0")

	app := New()
	app.EnableMCP()

	require.Nil(t, app.mcpServer)
	assert.True(t, app.bindMCPServer(t.Context()))
}

// TestMCPServer_Run_ServesOnTheBoundListener pins that Run serves the listener bind claimed, rather
// than binding a second time from the address.
func TestMCPServer_Run_ServesOnTheBoundListener(t *testing.T) {
	m := newMCPServer(testutil.GetFreePort(t), http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("mcp-up")) }))

	require.NoError(t, m.bind(t.Context()))

	done := make(chan struct{})

	go func() {
		defer close(done)

		m.Run(container.NewContainer(config.NewMockConfig(map[string]string{"LOG_LEVEL": "ERROR"})))
	}()

	require.Eventually(t, func() bool {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", m.port)) //nolint:noctx // readiness probe
		if err != nil {
			return false
		}

		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		return string(body) == "mcp-up"
	}, 5*time.Second, 20*time.Millisecond)

	require.NoError(t, m.Shutdown(t.Context()))

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("Run did not return after Shutdown; startMCPServer would hang on the waitgroup")
	}
}

// TestMCPServer_Run_WithoutBindRefusesToServe guards the ordering the design depends on. Run is only
// ever reached after bindMCPServer succeeds, so an unbound server reaching it means the sequence in
// (*App).Run was broken — it must say so rather than silently serve nothing or panic.
func TestMCPServer_Run_WithoutBindRefusesToServe(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		c := container.NewContainer(config.NewMockConfig(map[string]string{"LOG_LEVEL": "ERROR"}))
		newMCPServer(9999, http.NotFoundHandler()).Run(c)
	})

	assert.Contains(t, logs, "was not bound")
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

// TestMCPPort_Resolution covers every branch of mcpPort. The values that matter here are the ones an
// operator actually mistypes: a port with an extra digit, a pasted URL fragment, and the several
// spellings of zero that all mean "off".
func TestMCPPort_Resolution(t *testing.T) {
	tests := []struct {
		name     string
		port     string
		wantPort int
		wantOK   bool
		// wantLog is matched against stdout for the disable notice (Logf) and against stderr for the
		// fold warnings (Errorf); onStderr says which.
		wantLog  string
		onStderr bool
	}{
		{name: "unset falls back to the default", port: "", wantPort: defaultMCPPort, wantOK: true},
		{name: "explicit port is honored", port: "9310", wantPort: 9310, wantOK: true},
		{name: "max valid port is honored", port: "65535", wantPort: 65535, wantOK: true},
		{name: "min valid port is honored", port: "1", wantPort: 1, wantOK: true},

		// The disable switch, in the spellings that reach a config file. Before the parse-then-compare
		// change these fell through to the default and silently enabled the transport on 8200.
		{name: "zero disables", port: "0", wantLog: "MCP server is disabled"},
		{name: "padded zero disables", port: "00", wantLog: "MCP server is disabled"},
		{name: "signed zero disables", port: "+0", wantLog: "MCP server is disabled"},
		{name: "whitespace around zero disables", port: " 0 ", wantLog: "MCP server is disabled"},

		// Out of range: net.Listen rejects these permanently, which is a different failure from a busy
		// port and must not abort the whole service.
		{name: "extra digit folds to the default", port: "99999", wantPort: defaultMCPPort, wantOK: true,
			wantLog: "outside the valid port range", onStderr: true},
		{name: "negative folds to the default", port: "-1", wantPort: defaultMCPPort, wantOK: true,
			wantLog: "outside the valid port range", onStderr: true},

		// Not a number at all.
		{name: "typo folds to the default", port: "82oo", wantPort: defaultMCPPort, wantOK: true,
			wantLog: "is not a number", onStderr: true},
		{name: "pasted host:port folds to the default", port: "127.0.0.1:8200", wantPort: defaultMCPPort,
			wantOK: true, wantLog: "is not a number", onStderr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.NewServerConfigs(t)
			t.Setenv("MCP_PORT", tt.port)

			var (
				gotPort int
				gotOK   bool
			)

			capture := testutil.StdoutOutputForFunc
			if tt.onStderr {
				capture = testutil.StderrOutputForFunc
			}

			logs := capture(func() {
				app := New()
				gotPort, gotOK = app.mcpPort()
			})

			assert.Equal(t, tt.wantPort, gotPort)
			assert.Equal(t, tt.wantOK, gotOK)

			if tt.wantLog != "" {
				assert.Contains(t, logs, tt.wantLog)
			}
		})
	}
}

// TestEnableMCP_OutOfRangePortDoesNotAbortStartup is the regression test for the fold.
//
// An out-of-range MCP_PORT used to reach net.Listen, which rejects it permanently rather than
// transiently. bindMCPServer treats any bind error as fatal, so a single extra digit stopped the
// entire service - every transport, not just MCP - from starting at all, and the emitted remedy
// talked about port occupancy, which was not the problem.
func TestEnableMCP_OutOfRangePortDoesNotAbortStartup(t *testing.T) {
	testutil.NewServerConfigs(t)
	t.Setenv("MCP_PORT", "99999")

	app := New()
	app.GET("/ping", func(*Context) (any, error) { return "pong", nil })
	app.EnableMCP()

	require.NotNil(t, app.mcpServer)
	assert.Equal(t, defaultMCPPort, app.mcpServer.port,
		"an unbindable port must fold to the default, not be handed to net.Listen")
}

// TestBindMCPServer_CanceledContextReportsGracefulShutdown pins the other half of the bind-failure
// message. ListenConfig.Listen honors cancellation, so a SIGINT landing in the bind window fails
// the bind with context.Canceled - an operator stopping the process, not a port conflict. Reporting
// the port remedy for it sends them hunting a conflict that does not exist.
func TestBindMCPServer_CanceledContextReportsGracefulShutdown(t *testing.T) {
	testutil.NewServerConfigs(t)
	t.Setenv("MCP_PORT", strconv.Itoa(testutil.GetFreePort(t)))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var proceed bool

	logs := testutil.StdoutOutputForFunc(func() {
		app := New()
		app.GET("/ping", func(*Context) (any, error) { return "pong", nil })
		app.EnableMCP()

		proceed = app.bindMCPServer(ctx)
	})

	assert.False(t, proceed)
	assert.Contains(t, logs, "Startup canceled by context")
	assert.NotContains(t, logs, "Set MCP_PORT to a free port",
		"a canceled bind is not a port conflict and must not suggest the port remedy")
}

// TestBindMCPServer_FailedBindReleasesDatasources pins the cleanup, which is otherwise invisible:
// the run is abandoned before any server starts, but the startup hooks have already run and the
// container's datasources are open, and Run returns normally rather than exiting. Without the
// shutdown they would be left to process exit.
//
// "Application shutdown complete" is the last thing (*App).Shutdown logs, so its presence is what
// distinguishes a cleaned-up abort from a bare return.
func TestBindMCPServer_FailedBindReleasesDatasources(t *testing.T) {
	testutil.NewServerConfigs(t)

	occupied, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { _ = occupied.Close() })

	t.Setenv("MCP_PORT", strconv.Itoa(occupied.Addr().(*net.TCPAddr).Port))

	var proceed bool

	// Shutdown logs at INFO, which goes to stdout — the bind error itself goes to stderr and is
	// asserted by TestBindMCPServer_OccupiedPortAbortsStartup.
	logs := testutil.StdoutOutputForFunc(func() {
		app := New()
		app.GET("/ping", func(*Context) (any, error) { return "pong", nil })
		app.EnableMCP()

		proceed = app.bindMCPServer(t.Context())
	})

	require.False(t, proceed)
	assert.Contains(t, logs, "Application shutdown complete",
		"a failed bind must release what startup opened, not return and leave it to process exit")
}
