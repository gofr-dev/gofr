package gofr

import (
	ctx "context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/context"

	"gofr.dev/pkg"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/gofr/responder"
	"gofr.dev/pkg/log"
	"gofr.dev/pkg/middleware"
	"gofr.dev/pkg/middleware/oauth"
)

type server struct {
	contextPool sync.Pool
	mws         []Middleware
	mwVars      map[string]string

	done chan bool

	Router     Router
	HTTP       HTTP
	HTTPS      HTTPS
	GRPC       GRPC
	WSUpgrader websocket.Upgrader

	MetricsPort   int
	MetricsRoute  string
	metricsServer *http.Server

	// ValidateHeaders is used to decide if we need to enforce v3 headers and headers configured using VALIDATE_HEADERS
	// Making this false will disable this check. By default, it is set to false.
	ValidateHeaders bool
}

type HTTP struct {
	Port            int
	RedirectToHTTPS bool
}

const (
	defaultMetricsPort  = 2121
	defaultMetricsRoute = "/metrics"
)

// NewServer creates a new server instance with initialized properties, middleware, and configuration based on the provided parameters.
//
//	It sets up various middleware for logging, tracing, CORS handling, Prometheus metrics, and authentication.
//	The function returns the configured server instance.
//
//nolint:revive // We do not want anyone using the struct without initialization steps.
func NewServer(c Config, gofr *Gofr) *server {
	s := &server{
		Router:          NewRouter(),
		HTTP:            HTTP{},
		HTTPS:           HTTPS{},
		mwVars:          getMWVars(c),
		WSUpgrader:      websocket.Upgrader{},
		done:            make(chan bool),
		ValidateHeaders: false,
		MetricsPort:     defaultMetricsPort,
		MetricsRoute:    defaultMetricsRoute,
	}

	s.contextPool.New = func() interface{} {
		return NewContext(nil, nil, gofr)
	}

	// IMPORTANT: Following middleware will have to be added at initialization,
	// else our router's serveHTTP will not have context without calling start;
	// which means we will have to start server before running tests on responses.

	// Logger has to come after trace, for traceID to be logged.
	// contextInjector has to come after logging, else srw (custom response writer,
	// ref: logging middleware) will not be used and status code will not be logged.

	// Add NewRelic based on Config
	appName := c.Get("APP_NAME")
	appVersion := c.Get("APP_VERSION")
	tracerExporter := c.Get("TRACER_EXPORTER")
	nrLicense := c.Get("NEWRELIC_LICENSE")

	if appName != "" && nrLicense != "" {
		s.Router.Use(middleware.NewRelic(appName, nrLicense))
	}

	s.Router.Use(s.wsConnCreate)
	s.Router.Use(s.serverPushFlush)
	s.Router.Use(middleware.PropagateHeaders)
	s.Router.Use(s.removePathParamValueFromTraces())
	s.Router.Use(middleware.Trace(appName, appVersion, tracerExporter))
	s.Router.Use(middleware.CORS(s.mwVars))
	s.Router.Use(middleware.Logging(gofr.Logger, s.mwVars["LOG_OMIT_HEADERS"]))
	s.Router.Use(middleware.PrometheusMiddleware)

	s.setupAuth(c, gofr)

	return s
}

func (s *server) setupAuth(c Config, gofr *Gofr) {
	// OAuth
	if oAuthOptions, oAuthOk := getOAuthOptions(c); oAuthOk {
		if c.Get("LDAP_ADDR") != "" {
			gofr.Logger.Warn("OAuth middleware not enabled due to LDAP_ADDR env variable set")
			return
		}

		s.Router.Use(oauth.Auth(gofr.Logger, oAuthOptions))
	}
}

func (s *server) handleMetrics(l log.Logger) {
	if s.HTTP.Port == s.MetricsPort {
		if r, ok := s.Router.(*router); ok {
			l.Infof("Metrics server will run at :%v", s.HTTP.Port)
			r.Router.Handle(s.MetricsRoute, promhttp.Handler())
		}
	} else {
		// Start metrics server
		s.metricsServer = metricsServer(l, s.MetricsPort, s.MetricsRoute)
	}
}

// Start configures and starts the HTTP server based on provided settings.
// It handles routes for health checks, OpenAPI documentation, and Swagger UI.
// The method also manages middleware, logging, and gracefully shuts down the server when necessary, including
// redirecting HTTP to HTTPS if specified.
// The server starts both HTTP and GRPC servers, listens for incoming connections, and gracefully shuts down when
// signaled, ensuring proper cleanup.

//nolint:gocognit // reducing the cognitive complexity reduces the readability
func (s *server) Start(logger log.Logger) {
	s.Router.Route(http.MethodGet, pkg.PathHealthCheck, HealthHandler)
	s.Router.Route(http.MethodGet, pkg.PathHeartBeat, HeartBeatHandler)

	// check if openapi file is present
	if _, err := os.Stat("./api/openapi.json"); err == nil {
		s.Router.Route(http.MethodGet, pkg.PathOpenAPI, OpenAPIHandler)

		// routes for swagger-endpoints.
		s.Router.Route(http.MethodGet, pkg.PathSwagger, SwaggerUIHandler)
		s.Router.Route(http.MethodGet, pkg.PathSwaggerWithPathParam, SwaggerUIHandler)
	}

	s.handleMetrics(logger)

	if s.ValidateHeaders {
		s.Router.Use(middleware.ValidateHeaders(s.mwVars["VALIDATE_HEADERS"], logger))
	}

	// call the recovery middleware
	s.Router.Use(middleware.Recover(logger))

	// Use all user defined Middleware
	if len(s.mws) > 0 {
		s.Router.Use(s.mws...)
	}
	// moving context injector as the last added mw to allow  custom middlewares
	// to make changes in the request context.
	s.Router.Use(s.contextInjector)
	// Catch all route to ensure middleware are run for 404 routes - limitation of gorilla mux router
	s.Router.CatchAllRoute(func(c *Context) (i interface{}, err error) {
		// adding extra space to find exact route from routes string.
		path := fmt.Sprintf("%s ", c.Request().URL.Path)
		if strings.Contains(fmt.Sprint(s.Router), path) {
			return nil, &errors.Response{
				StatusCode: http.StatusMethodNotAllowed,
				Code:       "Invalid Method",
				Reason:     fmt.Sprintf("%v method not allowed for Route %v", c.Request().Method, c.req),
			}
		}

		return nil, &errors.Response{StatusCode: http.StatusNotFound, Code: "Invalid Route", Reason: fmt.Sprintf("Route %v not found", c.req)}
	})

	// logs all the routes of the server along with methods
	logger.Log(fmt.Sprint(s.Router))

	// Start HTTPS Server if key is present
	if s.HTTPS.KeyFile != "" && s.HTTPS.CertificateFile != "" {
		go s.HTTPS.StartServer(logger, s.Router)
	}

	// start the GRPC server, if the port is set
	if s.GRPC.Port != 0 {
		go s.GRPC.Start(logger)
	}

	// Start HTTP server. If redirection required, use the redirectHandler.
	var srv *http.Server

	go func() {
		var err error

		if s.HTTP.RedirectToHTTPS {
			logger.Logf("starting http redirect server at :%v", s.HTTP.Port)

			//nolint:gosec // not setting readheader timeout.
			srv = &http.Server{
				Addr:    ":" + strconv.Itoa(s.HTTP.Port),
				Handler: http.HandlerFunc(s.redirectHandler),
			}
			err = srv.ListenAndServe()
		} else {
			addr := ":" + strconv.Itoa(s.HTTP.Port)
			logger.Logf("starting http server at %s", addr)
			//nolint:gosec // noreadtimeoout will be set as of now.
			srv = &http.Server{
				Addr:    addr,
				Handler: s.Router,
			}
			err = srv.ListenAndServe()
		}

		if err != nil {
			s.done <- true
			logger.Errorf("error in starting http server at %v: %s", s.HTTP.Port, err)
		}
	}()

	<-s.done
	logger.Log("Server received on done channel. Stopping")

	const timeoutDuration = 5
	timeoutCtx, _ := context.WithTimeout(context.Background(), timeoutDuration*time.Second)
	_ = srv.Shutdown(timeoutCtx)
}

// Done gracefully shuts down the metrics server (if present) and signals completion by sending a message through the done channel.
func (s *server) Done() {
	if s.metricsServer != nil {
		_ = s.metricsServer.Shutdown(context.Background())
	}
	s.done <- true
}

// UseMiddleware is a setter method for passing user defined custom middleware
func (s *server) UseMiddleware(mws ...Middleware) {
	if s.mws != nil {
		s.mws = append(s.mws, mws...)

		return
	}

	s.mws = mws
}

type contextKey int

const gofrContextkey contextKey = 1
const appData middleware.LogDataKey = "appLogData"

// contextInjector injects *Context variable into every request using a middleware
func (s *server) contextInjector(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := s.contextPool.Get().(*Context)
		c.reset(responder.NewContextualResponder(w, r), request.NewHTTPRequest(r))
		*r = *r.Clone(ctx.WithValue(r.Context(), appData, &sync.Map{}))
		c.Context = r.Context()
		*r = *r.Clone(ctx.WithValue(c.Context, gofrContextkey, c))

		correlationID := r.Header.Get("X-B3-TraceID")
		if correlationID == "" {
			correlationID = r.Header.Get("X-Correlation-ID")
		}

		if correlationID == "" {
			correlationID = trace.SpanFromContext(r.Context()).SpanContext().TraceID().String()
		}

		c.Logger = log.NewCorrelationLogger(correlationID)

		inner.ServeHTTP(w, r)

		s.contextPool.Put(c)
	})
}

func (s *server) wsConnCreate(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := s.contextPool.Get().(*Context)
		c.WebSocketConnection = nil
		if r.Header.Get("Upgrade") == "websocket" {
			c.WebSocketConnection, _ = s.WSUpgrader.Upgrade(w, r, nil)
		}

		s.contextPool.Put(c)
		inner.ServeHTTP(w, r)
	})
}

// RedirectHandler redirects all http requests to https
func (s *server) redirectHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Connection", "close")

	Host := strings.Split(r.Host, ":")[0]
	path := "https://" + Host + ":" + strconv.Itoa(s.HTTPS.Port) + r.URL.String()

	// sets the Strict-Transport-Security policy field parameter. It forces the connection over HTTPS encryption
	w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
	http.Redirect(w, r, path, http.StatusMovedPermanently)
}

func getMWVars(c Config) (result map[string]string) {
	result = make(map[string]string)

	corsHeaders := middleware.AllowedCORSHeader()
	for _, v := range corsHeaders {
		if val := c.Get(v); val != "" {
			result[v] = val
		}
	}

	result["LOG_OMIT_HEADERS"] = c.Get("LOG_OMIT_HEADERS")

	// list of headers to be validated
	if val := c.Get("VALIDATE_HEADERS"); val != "" {
		result["VALIDATE_HEADERS"] = val
	}

	return
}

func getOAuthOptions(c Config) (options oauth.Options, ok bool) {
	options = oauth.Options{}
	if JWKPath := c.Get("JWKS_ENDPOINT"); JWKPath != "" {
		options.JWKPath = JWKPath
		ok = true
		// setting valid frequency to 30 mins if not provided in the config.
		if validFrequency, err := strconv.Atoi(c.Get("OAUTH_CACHE_VALIDITY")); err != nil {
			options.ValidityFrequency = 1800
		} else {
			options.ValidityFrequency = validFrequency
		}
	} else {
		ok = false
	}

	return
}

func (s *server) serverPushFlush(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := s.contextPool.Get().(*Context)
		c.ServerPush, _ = w.(http.Pusher)
		c.ServerFlush, _ = w.(http.Flusher)

		s.contextPool.Put(c)

		inner.ServeHTTP(w, r)
	})
}

func (s *server) removePathParamValueFromTraces() func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			routes := fmt.Sprint(s.Router)
			routeMap := filterValidRoutes(routes)

			if route, ok := isEndpointWithPathParam(routeMap, r.URL.Path); ok {
				//nolint // cannot create custom type as it will result in import cycle
				*r = *r.Clone(ctx.WithValue(r.Context(), "path", route))
			}

			inner.ServeHTTP(w, r)
		})
	}
}

func filterValidRoutes(routes string) map[string]bool {
	routeListWithMethod := strings.Split(routes, " ")
	routeMap := make(map[string]bool)

	for _, val := range routeListWithMethod {
		if strings.HasPrefix(val, "/") && !strings.HasPrefix(val, "/.well-known") {
			routeMap[val] = true
		}
	}

	return routeMap
}

func isEndpointWithPathParam(givenEndpoints map[string]bool, actualEndpoint string) (string, bool) {
	for pattern := range givenEndpoints {
		// Replace path parameters with a wildcard pattern
		patternWithoutParams := regexp.QuoteMeta(pattern)
		patternWithoutParams = regexp.MustCompile(`\\{[^/]+\\}`).ReplaceAllString(patternWithoutParams, `.*`)

		matched, err := regexp.MatchString("^"+patternWithoutParams+"$", actualEndpoint)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		if matched {
			return pattern, true
		}
	}

	return "", false
}
