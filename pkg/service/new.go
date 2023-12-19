package service

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"gofr.dev/pkg"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

const basic = "Basic"

// Options allows the user set all the options needs for http service like auth, service level headers, caching and surge protection
type Options struct {
	Headers           map[string]string // this can be used to pass service level headers.
	NumOfRetries      int
	SkipQParamLogging bool
	*Auth
	*Cache
	*SurgeProtectorOption
}

// KeyGenerator provides ability to  the user that can use custom or own logic to Generate the key for HTTPCached
type KeyGenerator func(url string, params map[string]interface{}, headers map[string]string) string

// Auth stores the information related to authentication. One can either use basic auth or OAuth
type Auth struct {
	UserName string // if token is not sent then the username and password can be sent and the token will be generated by the framework
	Password string
	*OAuthOption
}

// Cache provides the options needed for caching of HTTPService responses
type Cache struct {
	Cacher
	TTL          time.Duration
	KeyGenerator KeyGenerator
}

type SurgeProtectorOption struct {
	HeartbeatURL   string
	RetryFrequency int // indicates the time in seconds
	Disable        bool
}

//nolint:gochecknoglobals,gocritic// The declared global variable can be accessed across multiple functions
var (
	httpServiceResponse = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    pkg.FrameworkMetricsPrefix + "http_service_response",
		Help:    "Histogram of HTTP response times in seconds and status",
		Buckets: []float64{.001, .003, .005, .01, .025, .05, .1, .2, .3, .4, .5, .75, 1, 2, 3, 5, 10, 30},
	}, []string{"path", "method", "status"})

	circuitOpenCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: pkg.FrameworkMetricsPrefix + "external_service_circuit_open_count",
		Help: "Counter to track the number of times circuit opens",
	}, []string{"host"})
)

// NewHTTPServiceWithOptions creates a http client based on the options configured
//
//nolint:revive // returns an exported type because users should not be allowed to access this without initialization
func NewHTTPServiceWithOptions(resourceAddr string, logger log.Logger, options *Options) *httpService {
	// Register the prometheus metric
	if resourceAddr == "" {
		logger.Errorf("value for resourceAddress is empty")
	} else {
		resourceAddr = strings.TrimRight(resourceAddr, "/")
	}

	_ = prometheus.Register(httpServiceResponse)

	// Transport for http Client
	transport := otelhttp.NewTransport(http.DefaultTransport)

	httpSvc := &httpService{
		url:       resourceAddr,
		logger:    logger,
		Client:    &http.Client{Transport: transport, Timeout: RetryFrequency * time.Second}, // default timeout is 5 seconds
		isHealthy: true,
		healthCh:  make(chan bool),
		sp: surgeProtector{
			isEnabled:             true,
			customHeartbeatURL:    "/.well-known/heartbeat",
			retryFrequencySeconds: RetryFrequency,
			logger:                logger,
		},
	}

	if options == nil {
		httpSvc.SetSurgeProtectorOptions(true, httpSvc.sp.customHeartbeatURL, httpSvc.sp.retryFrequencySeconds)
		return httpSvc
	}

	httpSvc.skipQParamLogging = options.SkipQParamLogging

	// enable retries for call
	httpSvc.numOfRetries = options.NumOfRetries

	// enable service level headers
	if options.Headers != nil {
		httpSvc.customHeaders = options.Headers
	}

	httpSvc.initializeClientWithAuth(*options)

	enableSP := true

	// enable surge protection
	if options.SurgeProtectorOption != nil {
		if options.RetryFrequency != 0 {
			httpSvc.sp.retryFrequencySeconds = options.RetryFrequency
		}

		if options.HeartbeatURL != "" {
			httpSvc.sp.customHeartbeatURL = options.HeartbeatURL
		}

		enableSP = !options.SurgeProtectorOption.Disable
	}

	httpSvc.SetSurgeProtectorOptions(enableSP, httpSvc.sp.customHeartbeatURL, httpSvc.sp.retryFrequencySeconds)

	// enable http service with cache
	if options.Cache != nil {
		httpSvc.cache = &cachedHTTPService{
			httpService:  httpSvc,
			cacher:       options.Cacher,
			ttl:          options.TTL,
			keyGenerator: options.KeyGenerator,
		}
	}

	return httpSvc
}

func (h *httpService) initializeClientWithAuth(options Options) {
	if options.Auth == nil {
		return
	}

	// simple auth
	if options.UserName != "" && options.OAuthOption == nil { // OAuth and basic auth cannot co-exist
		h.isSet = true
		h.auth = basic + " " + base64.StdEncoding.EncodeToString([]byte(options.UserName+":"+options.Password))
	}

	h.enableOAuth(options)
}

func (h *httpService) enableOAuth(options Options) {
	if options.OAuthOption != nil && h.auth == "" { // if auth is already set to basic auth, dont set oauth
		h.isSet = true

		if options.TimeBeforeExpiryToRefresh == 0 {
			options.TimeBeforeExpiryToRefresh = oAuthExpiryBeforeTime
		}

		h.isTokenGenBlocking = options.WaitForTokenGen
		h.isTokenPresent = make(chan bool)

		// WaitForTokenGen is a flag indicating if the token generation is blocking or not.
		// if the call is tokenGen blocking, we read from the channel(in pre call) to make sure token generation is complete
		// we close the channel as soon as we get the first token,
		// as we don't have to make it a blocking a call once the token is received
		go func() {
			h.setClientOauthHeader(options.OAuthOption)
			close(h.isTokenPresent)
		}()
	}
}

// HealthCheck performs a health check and returns the health status based on whether the service is considered healthy or not.
func (h *httpService) HealthCheck() types.Health {
	h.mu.Lock()
	isHealthy := h.isHealthy
	h.mu.Unlock()

	if isHealthy {
		return types.Health{
			Name:   h.url,
			Status: pkg.StatusUp,
		}
	}

	return types.Health{
		Name:   h.url,
		Status: pkg.StatusDown,
	}
}
