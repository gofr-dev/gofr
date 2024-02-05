package logging

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"gofr.dev/pkg/gofr/service"
)

const levelFetchInterval = 5

func NewRemoteLogger(level Level, remoteConfigURL string) Logger {
	l := remoteLogger{
		remoteURL:    remoteConfigURL,
		Logger:       NewLogger(level),
		currentLevel: level,
	}

	if remoteConfigURL != "" {
		go l.UpdateLogLevel()
	}

	return l
}

type remoteLogger struct {
	remoteURL    string
	currentLevel Level
	Logger
}

func (r *remoteLogger) UpdateLogLevel() {
	ticker := time.NewTicker(levelFetchInterval * time.Minute)
	defer ticker.Stop()

	remoteService := service.NewHTTPService(r.remoteURL, r.Logger)

	for range ticker.C {
		newLevel, err := fetchAndUpdateLogLevel(remoteService)
		if err == nil {
			r.changeLevel(newLevel)
			r.currentLevel = newLevel
		}
	}
}

func fetchAndUpdateLogLevel(remoteService service.HTTP) (Level, error) {
	resp, err := remoteService.Get(context.Background(), "", nil)
	if err != nil {
		return INFO, err
	}
	defer resp.Body.Close()

	var response struct {
		Data []struct {
			ServiceName string            `json:"serviceName"`
			Level       map[string]string `json:"logLevel"`
		} `json:"data"`
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return INFO, err
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return INFO, err
	}

	if len(response.Data) > 0 {
		newLevel := GetLevelFromString(response.Data[0].Level["LOG_LEVEL"])
		return newLevel, nil
	}

	return INFO, nil
}
