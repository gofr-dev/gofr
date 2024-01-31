package logging

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

const levelFetchInterval = 5

type remoteLevelService struct {
	url             string
	accessKey       string
	appName         string
	logLevel        Level
	logger          *logger
	ticker          *time.Ticker
	logLevelChannel chan Level
}

// updateLogLevel continuously fetches and updates the log level from the remote service.
func (rl *remoteLevelService) updateLogLevel() {
	defer rl.ticker.Stop()

	for {
		select {
		case <-rl.ticker.C:
			newLevel, err := rl.fetchLogLevel()
			if err != nil {
				rl.logger.Errorf("Failed to fetch remote log level: %v", err)
				continue
			}

			// Send the new log level to the channel
			rl.logLevelChannel <- newLevel

		case newLevel := <-rl.logLevelChannel:
			// Update the logger's log level based on the fetched value.
			rl.logger.level = newLevel
		}
	}
}

// fetchLogLevel fetches the log level from the remote logging service.
func (rl *remoteLevelService) fetchLogLevel() (Level, error) {
	client := &http.Client{
		Timeout: 5 * time.Second, // Add timeout for request
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", rl.url, http.NoBody)
	if err != nil {
		rl.logger.Errorf("Failed to fetch remote log level: %v", err)
		return rl.logLevel, err
	}

	req.Header.Set("Access-Key", rl.accessKey)
	req.Header.Set("App-Name", rl.appName)

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode == http.StatusInternalServerError {
		return rl.logLevel, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return rl.logLevel, err
	}

	var response struct {
		Data []struct {
			ServiceName string            `json:"serviceName"`
			Level       map[string]string `json:"logLevel"`
		} `json:"data"`
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		rl.logger.Errorf("Logging Service returned %v", err)
	}

	if len(response.Data) > 0 {
		logLevel := response.Data[0].Level["LOG_LEVEL"]
		newLevel := GetLevelFromString(logLevel)

		return newLevel, nil
	}

	return rl.logLevel, nil
}
