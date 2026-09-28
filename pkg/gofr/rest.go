package gofr

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/trace"

	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
)

// MethodQuery is the HTTP QUERY method (RFC 10008). Declared here alongside the
// other route registrations so callers routing a request or writing middleware
// have the canonical constant next to App.QUERY. Aliases gofrHTTP.MethodQuery.
const MethodQuery = gofrHTTP.MethodQuery

// GET adds a Handler for HTTP GET method for a route pattern.
func (a *App) GET(pattern string, handler Handler) {
	a.add("GET", pattern, handler)
}

// PUT adds a Handler for HTTP PUT method for a route pattern.
func (a *App) PUT(pattern string, handler Handler) {
	a.add("PUT", pattern, handler)
}

// POST adds a Handler for HTTP POST method for a route pattern.
func (a *App) POST(pattern string, handler Handler) {
	a.add("POST", pattern, handler)
}

// DELETE adds a Handler for HTTP DELETE method for a route pattern.
func (a *App) DELETE(pattern string, handler Handler) {
	a.add("DELETE", pattern, handler)
}

// PATCH adds a Handler for HTTP PATCH method for a route pattern.
func (a *App) PATCH(pattern string, handler Handler) {
	a.add("PATCH", pattern, handler)
}

// QUERY adds a Handler for the HTTP QUERY method (RFC 10008) for a route pattern.
// QUERY is a safe, idempotent method that carries a request body describing the
// query; read it in the handler via ctx.Bind, the same way as a POST body.
// Per RFC 10008 a QUERY request MUST carry a Content-Type the server can interpret;
// GoFr enforces this on the QUERY path, rejecting a missing Content-Type with 400
// and an unsupported one with 415 before the handler runs. Other verbs, including
// POST, are unaffected.
func (a *App) QUERY(pattern string, handler Handler) {
	a.add(MethodQuery, pattern, handler)
}

func (a *App) add(method, pattern string, h Handler) {
	if !a.httpRegistered && !isPortAvailable(a.httpServer.port) {
		a.container.Logger.Fatalf("http port %d is blocked or unreachable", a.httpServer.port)
	}

	a.httpRegistered = true

	reqTimeout, err := strconv.Atoi(a.Config.Get("REQUEST_TIMEOUT"))
	if (err != nil && a.Config.Get("REQUEST_TIMEOUT") != "") || reqTimeout < 0 {
		reqTimeout = 0
	}

	var routeHandler http.Handler = handler{
		function:       h,
		container:      a.container,
		requestTimeout: time.Duration(reqTimeout) * time.Second,
	}

	// RFC 10008: a QUERY request MUST carry a Content-Type the server can
	// interpret. Guard only registered QUERY routes — wrapping at ServeHTTP
	// would gate the PathPrefix("/") catch-all too, turning a real 404 into
	// a spurious 400/415 for QUERY to any unknown path.
	if method == MethodQuery {
		routeHandler = queryContentTypeGuard(routeHandler, a.container.Logger)
	}

	a.httpServer.router.Add(method, pattern, routeHandler)
}

// queryContentTypeGuard enforces RFC 10008's Content-Type requirement on
// QUERY requests before the user handler runs. Missing → 400, unsupported →
// 415. A denial is logged with the request's trace ID and dispatched to the
// log level the error itself declares — ErrorMissingParam and
// ErrorUnsupportedMediaType both return LogLevel INFO, so a client's bad
// header does NOT surface as an ERROR line and does not page the on-call.
// This mirrors handler.logError so the two paths cannot diverge.
//
// A 415 also carries the Accept-Query response header per RFC 10008 §3.1, so
// the client learns which media types the server would have accepted. The set
// is derived from bindersByMediaType (via gofrHTTP.AcceptedQueryMediaTypes),
// so the advertised list cannot drift from the set Bind actually decodes.
func queryContentTypeGuard(inner http.Handler, log logging.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := gofrHTTP.ValidateQueryContentType(r); err != nil {
			traceID := trace.SpanFromContext(r.Context()).SpanContext().TraceID().String()
			entry := &ErrorLogEntry{TraceID: traceID, Error: err.Error()}

			logHelper := log.Error

			switch logging.GetLogLevelForError(err) {
			case logging.ERROR:
				// default is log.Error
			case logging.INFO:
				logHelper = log.Info
			case logging.NOTICE:
				logHelper = log.Notice
			case logging.DEBUG:
				logHelper = log.Debug
			case logging.WARN:
				logHelper = log.Warn
			case logging.FATAL:
				logHelper = log.Fatal
			}

			logHelper(entry)

			var unsupported gofrHTTP.ErrorUnsupportedMediaType
			if errors.As(err, &unsupported) {
				w.Header().Set("Accept-Query", gofrHTTP.AcceptedQueryMediaTypes())
			}

			gofrHTTP.NewResponder(w, r.Method).Respond(nil, err)

			return
		}

		inner.ServeHTTP(w, r)
	})
}

// AddRESTHandlers creates and registers CRUD routes for the given struct, the struct should always be passed by reference.
func (a *App) AddRESTHandlers(object any) error {
	cfg, err := scanEntity(object)
	if err != nil {
		a.container.Logger.Errorf("%v", err)
		return err
	}

	a.registerCRUDHandlers(cfg, object)

	return nil
}
