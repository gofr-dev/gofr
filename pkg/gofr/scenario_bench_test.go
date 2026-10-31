package gofr

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/internal/stablemw"
)

// scenarioUser is the JSON body the bind scenario decodes and echoes back.
type scenarioUser struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

// scenarioItem is the small struct the JSON scenario encodes.
type scenarioItem struct {
	ID     int      `json:"id"`
	Name   string   `json:"name"`
	Active bool     `json:"active"`
	Tags   []string `json:"tags"`
}

const scenarioBindBody = `{"name":"x","age":30,"email":"x@y.z"}`

// newScenarioApp builds an App the way a user gets one from New(), registers
// the benchmark routes and finishes the setup Run would do before serving, so
// the returned handler is exactly what the HTTP server serves. logLevel "" is
// the user default; "ERROR" is the minimal-observability mode.
//
// Request logs go to os.Stdout, so stdout is pointed at /dev/null for the run:
// the write syscall stays in the measurement, the terminal output does not.
func newScenarioApp(b *testing.B, logLevel string, middlewares int) http.Handler {
	b.Helper()

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		b.Fatal(err)
	}

	stdout := os.Stdout
	os.Stdout = devNull

	b.Cleanup(func() {
		os.Stdout = stdout

		devNull.Close()
	})

	b.Setenv("HTTP_PORT", "0")
	b.Setenv("METRICS_PORT", "0")

	if logLevel != "" {
		b.Setenv("LOG_LEVEL", logLevel)
	}

	app := New()

	for i := range middlewares {
		header := "X-Mw-" + string(rune('1'+i))

		app.UseMiddleware(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set(header, "1")
				next.ServeHTTP(w, r)
			})
		})
	}

	app.GET("/plaintext", func(*Context) (any, error) { return "Hello, World!", nil })
	app.GET("/json", func(*Context) (any, error) {
		return scenarioItem{ID: 1, Name: "gofr", Active: true, Tags: []string{"a", "b"}}, nil
	})
	app.GET("/user/{id}", func(c *Context) (any, error) {
		return struct {
			ID string `json:"id"`
		}{ID: c.PathParam("id")}, nil
	})
	app.POST("/bind", func(c *Context) (any, error) {
		var u scenarioUser
		if err := c.Bind(&u); err != nil {
			return nil, err
		}

		return u, nil
	})

	app.httpServerSetup()
	// As run does: the websocket upgrade is registered last, with GoFr's mark.
	app.httpServer.router.Use(stablemw.Mark(middleware.WSHandlerUpgrade(app.container, app.httpServer.ws)))

	return app.httpServer.router
}

func benchmarkScenario(b *testing.B, logLevel string, middlewares int, method, target, body string) {
	b.Helper()

	h := newScenarioApp(b, logLevel, middlewares)

	// One request is reused, as the repo's other full-chain benchmarks do, so the
	// numbers are GoFr's own cost rather than httptest's request parsing. GoFr
	// never mutates the request it is handed (middlewares derive copies), so
	// reuse is sound; the body reader is rewound for the bind scenario.
	var bodyReader *strings.Reader

	req := httptest.NewRequestWithContext(b.Context(), method, target, http.NoBody)

	if body != "" {
		bodyReader = strings.NewReader(body)
		req.Body = io.NopCloser(bodyReader)
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "application/json")
	}

	w := &benchDiscardResponseWriter{}

	serve := func() {
		if bodyReader != nil {
			bodyReader.Reset(body)
		}

		w.reset()
		h.ServeHTTP(w, req)
	}

	// Prove the work happens: the request must answer 2xx before timing.
	serve()

	if w.code/100 != 2 {
		b.Fatalf("%s %s answered %d", method, target, w.code)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		serve()
	}
}

// BenchmarkScenario runs the five load-test scenarios through the full GoFr
// handler (router, default middleware chain, handler, responder) in both
// observability modes, as the GoFr side of the load-test harness.
func BenchmarkScenario(b *testing.B) {
	modes := []struct{ name, logLevel string }{
		{"defaults", ""},
		{"minimised", "ERROR"}, //nolint:misspell // the mode name the recorded results use
	}

	scenarios := []struct {
		name, method, target, body string
		middlewares                int
	}{
		{"plaintext", http.MethodGet, "/plaintext", "", 0},
		{"json", http.MethodGet, "/json", "", 0},
		{"pathparam", http.MethodGet, "/user/42", "", 0},
		{"mw3", http.MethodGet, "/plaintext", "", 3},
		{"bind", http.MethodPost, "/bind", scenarioBindBody, 0},
	}

	for _, m := range modes {
		for _, s := range scenarios {
			b.Run(m.name+"/"+s.name, func(b *testing.B) {
				benchmarkScenario(b, m.logLevel, s.middlewares, s.method, s.target, s.body)
			})
		}
	}
}
