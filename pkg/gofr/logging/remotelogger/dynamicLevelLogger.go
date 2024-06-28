package remotelogger

import (
	"context"
	"encoding/json"
	"io"
	"slices"
	"strconv"
	"time"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
)

const (
	requestTimeout = 5 * time.Second
)

/*
New creates a new RemoteLogger instance with the provided level, remote configuration URL, and level fetch interval.
The remote configuration URL is expected to be a JSON endpoint that returns the desired log level for the service.
The level fetch interval determines how often the logger checks for updates to the remote configuration.
*/
func New(level logging.Level, remoteConfigURL, loggerFetchInterval string) logging.Logger {
	interval, err := strconv.Atoi(loggerFetchInterval)
	if err != nil {
		interval = 15
	}

	l := remoteLogger{
		remoteURL:          remoteConfigURL,
		Logger:             logging.NewLogger(level),
		levelFetchInterval: interval,
		currentLevel:       level,
	}

	if remoteConfigURL != "" {
		go l.UpdateLogLevel()
	}

	return l
}

type remoteLogger struct {
	remoteURL          string
	levelFetchInterval int
	currentLevel       logging.Level
	logging.Logger
}

// UpdateLogLevel continuously fetches the log level from the remote configuration URL at the specified interval
// and updates the underlying log level if it has changed.
func (r *remoteLogger) UpdateLogLevel() {
	interval := time.Duration(r.levelFetchInterval) * time.Second
	ticker := time.NewTicker(interval)

	defer ticker.Stop()

	remoteService := service.NewHTTPService(r.remoteURL, r.Logger, nil)

	for range ticker.C {
		newLevel, err := fetchAndUpdateLogLevel(remoteService, r.currentLevel)
		if err == nil {
			r.ChangeLevel(newLevel)

			if r.currentLevel != newLevel {
				r.Infof("LOG_LEVEL updated from %v to %v", r.currentLevel, newLevel)
				r.currentLevel = newLevel
			}
		}
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
