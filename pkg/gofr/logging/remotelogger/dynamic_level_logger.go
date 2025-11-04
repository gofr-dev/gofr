package remotelogger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
)

const (
	requestTimeout = 5 * time.Second
	// ANSI color codes for terminal output.
	colorBlue   = 34  // For successful responses (2xx)
	colorYellow = 220 // For client errors (4xx)
	colorRed    = 202 // For server errors (5xx)
)

// httpDebugMsg represents a structured HTTP debug log entry.
// It implements PrettyPrint for colored output and json.Marshaler for JSON logs.
type httpDebugMsg struct {
	CorrelationID string `json:"correlation_id"`
	ResponseCode  int    `json:"response_code"`
	ResponseTime  int64  `json:"response_time_us"`
	HTTPMethod    string `json:"http_method"`
	URI           string `json:"uri"`
}

func (m httpDebugMsg) PrettyPrint(w io.Writer) {
	colorCode := colorForResponseCode(m.ResponseCode)
	fmt.Fprintf(w,
		"\u001B[38;5;8m%s \u001B[38;5;%dm%-6d\u001B[0m %8dμs\u001B[0m %s %s\n",
		m.CorrelationID,
		colorCode,
		m.ResponseCode,
		m.ResponseTime,
		m.HTTPMethod,
		m.URI,
	)
}

func (m httpDebugMsg) MarshalJSON() ([]byte, error) {
	type alias httpDebugMsg
	return json.Marshal(alias(m))
}

// httpLogFilter filters HTTP logs from remote logger to reduce noise.
type httpLogFilter struct {
	logging.Logger
	mu                 sync.Mutex
	firstSuccessfulHit bool
	initLogged         bool
}

// Log implements a simplified filtering strategy with consistent formatting.
func (f *httpLogFilter) Log(args ...any) {
	if len(args) == 0 || args[0] == nil {
		f.Logger.Log(args...)
		return
	}

	// Handle HTTP logs.
	httpLog, ok := args[0].(*service.Log)
	if !ok {
		f.Logger.Log(args...)
		return
	}

	f.handleHTTPLog(httpLog, args)
}

func (f *httpLogFilter) handleHTTPLog(httpLog *service.Log, args []any) {
	// Log initialization message if not already logged
	f.mu.Lock()
	notLoggedYet := !f.initLogged

	if notLoggedYet {
		f.initLogged = true
	}

	f.mu.Unlock()

	if notLoggedYet {
		f.Logger.Infof("Initializing remote logger connection to %s", httpLog.URI)
	}

	isSuccessful := httpLog.ResponseCode >= 200 && httpLog.ResponseCode < 300

	f.mu.Lock()
	isFirstHit := !f.firstSuccessfulHit
	f.mu.Unlock()

	switch {
	// First successful hit - log at INFO level
	case isSuccessful && isFirstHit:
		f.mu.Lock()
		f.firstSuccessfulHit = true
		f.mu.Unlock()
		f.Logger.Log(args...)

	// Subsequent successful hits - log at DEBUG level with consistent format
	case isSuccessful:
		msg := httpDebugMsg{
			CorrelationID: httpLog.CorrelationID,
			ResponseCode:  httpLog.ResponseCode,
			ResponseTime:  httpLog.ResponseTime,
			HTTPMethod:    httpLog.HTTPMethod,
			URI:           httpLog.URI,
		}
		f.Logger.Debug(msg)

	// Error responses - pass through to original logger
	default:
		f.Logger.Log(args...)
	}
}

func colorForResponseCode(status int) int {
	switch {
	case status >= 200 && status < 300:
		return colorBlue
	case status >= 400 && status < 500:
		return colorYellow
	case status >= 500 && status < 600:
		return colorRed
	}

	return 0
}

/*
New creates a new RemoteLogger instance with the provided level, remote configuration URL, and level fetch interval.
The remote configuration URL is expected to be a JSON endpoint that returns the desired log level for the service.
The level fetch interval determines how often the logger checks for updates to the remote configuration.
*/
func New(level logging.Level, remoteConfigURL string, loggerFetchInterval time.Duration) logging.Logger {
	l := &remoteLogger{
		remoteURL:          remoteConfigURL,
		Logger:             logging.NewLogger(level),
		levelFetchInterval: loggerFetchInterval,
		currentLevel:       level,
	}

	if remoteConfigURL != "" {
		cfg := NewHTTPRemoteConfig(remoteConfigURL, loggerFetchInterval, l.Logger)
		cfg.Register(l)

		go cfg.Start()
	}

	return l
}

type remoteLogger struct {
	remoteURL          string
	levelFetchInterval time.Duration
	mu                 sync.RWMutex
	currentLevel       logging.Level
	logging.Logger
}

// UpdateConfig implements the config.RemoteConfigurable interface and updates the log level based on the provided configuration.
func (r *remoteLogger) UpdateConfig(cfg map[string]any) {
	if levelStr, ok := cfg["logLevel"].(string); ok {
		newLevel := logging.GetLevelFromString(levelStr)

		r.mu.Lock()

		if r.currentLevel != newLevel {
			oldLevel := r.currentLevel
			r.currentLevel = newLevel
			r.mu.Unlock()

			logLevelChange(r, oldLevel, newLevel)
			r.ChangeLevel(newLevel)
		} else {
			r.mu.Unlock()
		}
	}
}

// UpdateLogLevel continuously fetches the log level from the remote configuration URL at the specified interval
// and updates the underlying log level if it has changed.
func (r *remoteLogger) UpdateLogLevel() {
	// Create filtered logger with proper initialization
	filteredLogger := &httpLogFilter{
		Logger:             r.Logger,
		firstSuccessfulHit: false,
		initLogged:         false,
	}

	remoteService := service.NewHTTPService(r.remoteURL, filteredLogger, nil)

	r.Infof("Remote logger monitoring initialized with URL: %s, interval: %s",
		r.remoteURL, r.levelFetchInterval)

	checkAndUpdateLevel := func() {
		r.mu.RLock()
		currentLevel := r.currentLevel
		r.mu.RUnlock()

		newLevel, err := fetchAndUpdateLogLevel(remoteService, currentLevel)
		if err != nil {
			r.Warnf("Failed to fetch log level: %v", err)
			return
		}

		r.mu.Lock()

		if r.currentLevel != newLevel {
			oldLevel := r.currentLevel
			r.currentLevel = newLevel
			r.mu.Unlock()

			logLevelChange(r, oldLevel, newLevel)
			r.ChangeLevel(newLevel)
		} else {
			r.mu.Unlock()
		}
	}

	// Perform initial check immediately
	checkAndUpdateLevel()

	// Setup ticker for periodic checks
	ticker := time.NewTicker(r.levelFetchInterval)
	defer ticker.Stop()

	for range ticker.C {
		checkAndUpdateLevel()
	}
}

// Helper function to log level changes at appropriate level.
func logLevelChange(r *remoteLogger, oldLevel, newLevel logging.Level) {
	// Use the higher level to ensure visibility
	logLevel := oldLevel
	if newLevel > oldLevel {
		logLevel = newLevel
	}

	message := fmt.Sprintf("LOG_LEVEL updated from %v to %v", oldLevel, newLevel)

	switch logLevel {
	case logging.FATAL:
		r.Warnf(message)
	case logging.ERROR:
		r.Errorf(message)
	case logging.WARN:
		r.Warnf(message)
	case logging.NOTICE:
		r.Noticef(message)
	case logging.INFO:
		r.Infof(message)
	case logging.DEBUG:
		r.Infof(message) // Using Info for DEBUG to ensure visibility
	}
}

func fetchAndUpdateLogLevel(remoteService service.HTTP, currentLevel logging.Level) (logging.Level, error) {
	if newLogLevelStr, err := fetchLogLevelStr(remoteService); err != nil {
		return currentLevel, err
	} else if newLogLevelStr != "" {
		return logging.GetLevelFromString(newLogLevelStr), nil
	}

	return currentLevel, nil
}

func fetchLogLevelStr(remoteService service.HTTP) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout) // Set timeout for 5 seconds
	defer cancel()

	resp, err := remoteService.Get(ctx, "", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var response struct {
		Data struct {
			ServiceName string `json:"serviceName"`
			Level       string `json:"logLevel"`
		} `json:"data"`
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return "", err
	}

	logLevels := []string{"DEBUG", "INFO", "NOTICE", "WARN", "ERROR", "FATAL"}

	if slices.Contains(logLevels, response.Data.Level) && response.Data.ServiceName != "" {
		return response.Data.Level, nil
	}

	return "", nil
}

// httpRemoteConfig fetches runtime config via HTTP and pushes it to registered clients.
type httpRemoteConfig struct {
	url      string
	interval time.Duration
	logger   logging.Logger
	clients  []config.RemoteConfigurable
}

func NewHTTPRemoteConfig(url string, interval time.Duration, logger logging.Logger) config.RemoteConfiguration {
	return &httpRemoteConfig{
		url:      url,
		interval: interval,
		logger:   logger,
	}
}

func (h *httpRemoteConfig) Register(c config.RemoteConfigurable) {
	h.clients = append(h.clients, c)
}

func (h *httpRemoteConfig) Start() {
	filteredLogger := &httpLogFilter{Logger: h.logger}
	remoteService := service.NewHTTPService(h.url, filteredLogger, nil)

	h.logger.Infof("Remote configuration monitoring initialized with URL: %s, interval: %s",
		h.url, h.interval)

	checkAndUpdate := func() {
		config, err := fetchRemoteConfig(remoteService)
		if err != nil {
			h.logger.Warnf("Failed to fetch remote config: %v", err)
			return
		}

		for _, client := range h.clients {
			client.UpdateConfig(config)
		}
	}

	// run once immediately
	checkAndUpdate()

	// then periodically
	ticker := time.NewTicker(h.interval)

	go func() {
		defer ticker.Stop()

		for range ticker.C {
			checkAndUpdate()
		}
	}()
}

func fetchRemoteConfig(remoteService service.HTTP) (map[string]any, error) {
	if newLogLevelStr, err := fetchLogLevelStr(remoteService); err != nil {
		return map[string]any{}, err
	} else if newLogLevelStr != "" {
		return map[string]any{
			"logLevel": newLogLevelStr,
		}, nil
	}

	return map[string]any{}, nil
}
