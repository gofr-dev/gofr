package remotelogger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"time"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
)

const (
	requestTimeout = 5 * time.Second
)

// filteringLogger filters HTTP logs from remote logger to reduce noise
type filteringLogger struct {
	logging.Logger
	firstSuccessfulHit bool
	initLogged         bool
}

// Log implements a simplified filtering strategy with consistent formatting
func (f *filteringLogger) Log(args ...any) {
	if len(args) == 0 || args[0] == nil {
		f.Logger.Log(args...)
		return
	}

	// Handle HTTP logs
	httpLog, ok := args[0].(*service.Log)
	if !ok {
		f.Logger.Log(args...)
		return
	}

	// Log initialization message if not already logged
	if !f.initLogged {
		f.initLogged = true
		f.Logger.Infof("Initializing remote logger connection to %s", httpLog.URI)
	}

	isSuccessful := httpLog.ResponseCode >= 200 && httpLog.ResponseCode < 300

	switch {
	// First successful hit - log at INFO level
	case isSuccessful && !f.firstSuccessfulHit:
		f.firstSuccessfulHit = true
		f.Logger.Log(args...)

	// Subsequent successful hits - log at DEBUG level with consistent format
	case isSuccessful:
		if debugLogger, ok := f.Logger.(interface{ Debugf(string, ...any) }); ok {
			colorCode := colorForResponseCode(httpLog.ResponseCode)
			debugLogger.Debugf("\u001B[38;5;8m%s \u001B[38;5;%dm%-6d\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s %s",
				httpLog.CorrelationID,
				colorCode,
				httpLog.ResponseCode,
				httpLog.ResponseTime,
				httpLog.HTTPMethod,
				httpLog.URI)
		}

	// Error responses - pass through to original logger
	default:
		f.Logger.Log(args...)
	}
}

func colorForResponseCode(status int) int {
	switch {
	case status >= 200 && status < 300:
		return 34 // blue
	case status >= 400 && status < 500:
		return 220 // yellow
	case status >= 500 && status < 600:
		return 202 // red
	}
	return 0
}

/*
New creates a new RemoteLogger instance with the provided level, remote configuration URL, and level fetch interval.
The remote configuration URL is expected to be a JSON endpoint that returns the desired log level for the service.
The level fetch interval determines how often the logger checks for updates to the remote configuration.
*/
func New(level logging.Level, remoteConfigURL string, loggerFetchInterval time.Duration) logging.Logger {
	l := remoteLogger{
		remoteURL:          remoteConfigURL,
		Logger:             logging.NewLogger(level),
		levelFetchInterval: loggerFetchInterval,
		currentLevel:       level,
	}

	if remoteConfigURL != "" {
		go l.UpdateLogLevel()
	}

	return l
}

type remoteLogger struct {
	remoteURL          string
	levelFetchInterval time.Duration
	currentLevel       logging.Level
	logging.Logger
}

// UpdateLogLevel continuously fetches the log level from the remote configuration URL at the specified interval
// and updates the underlying log level if it has changed.
func (r *remoteLogger) UpdateLogLevel() {
	interval := r.levelFetchInterval
	ticker := time.NewTicker(interval)

	defer ticker.Stop()

	// Create filtered logger with proper initialization
	filteredLogger := &filteringLogger{
		Logger:             r.Logger,
		firstSuccessfulHit: false,
		initLogged:         false,
	}

	remoteService := service.NewHTTPService(r.remoteURL, filteredLogger, nil)

	r.Infof("Remote logger monitoring initialized with URL: %s, interval: %s",
		r.remoteURL, r.levelFetchInterval)

	checkAndUpdateLevel := func() {
		newLevel, err := fetchAndUpdateLogLevel(remoteService, r.currentLevel)
		if err != nil {
			r.Warnf("Failed to fetch log level: %v", err)
			return
		}

		if r.currentLevel != newLevel {
			logLevelChange(r, r.currentLevel, newLevel)
			r.ChangeLevel(newLevel)
			r.currentLevel = newLevel
		}
	}

	// Perform initial check immediately
	checkAndUpdateLevel()

	// Setup ticker for periodic checks
	ticker = time.NewTicker(r.levelFetchInterval)
	defer ticker.Stop()

	for range ticker.C {
		checkAndUpdateLevel()
	}
}

// Helper function to log level changes at appropriate level
func logLevelChange(r *remoteLogger, oldLevel, newLevel logging.Level) {
	// Use the higher level to ensure visibility
	logLevel := oldLevel
	if newLevel > oldLevel {
		logLevel = newLevel
	}

	message := fmt.Sprintf("LOG_LEVEL updated from %v to %v", oldLevel, newLevel)

	switch logLevel {
	case logging.FATAL:
		r.Fatalf(message)
	case logging.ERROR:
		r.Errorf(message)
	default:
		r.Infof(message)
	}
}

func fetchAndUpdateLogLevel(remoteService service.HTTP, currentLevel logging.Level) (logging.Level, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout) // Set timeout for 5 seconds
	defer cancel()

	resp, err := remoteService.Get(ctx, "", nil)
	if err != nil {
		return currentLevel, err
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
		return currentLevel, err
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return currentLevel, err
	}

	logLevels := []string{"DEBUG", "INFO", "NOTICE", "WARN", "ERROR", "FATAL"}

	if slices.Contains(logLevels, response.Data.Level) && response.Data.ServiceName != "" {
		newLevel := logging.GetLevelFromString(response.Data.Level)
		return newLevel, nil
	}

	return currentLevel, nil
}
