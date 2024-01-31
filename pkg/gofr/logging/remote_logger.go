package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const levelFetchInterval = 2

type RemoteLevelService struct {
	url             string
	accessKey       string
	appName         string
	LogLevel        Level
	logger          *logger
	ticker          *time.Ticker
	logLevelChannel chan Level
}

func (rl *RemoteLevelService) updateLogLevel() {
	defer rl.ticker.Stop()

	for {
		select {
		case <-rl.ticker.C:
			newLevel, err := rl.fetchLogLevel()
			fmt.Errorf("fetched the log level %s", newLevel)
			if err != nil {
				rl.logger.Errorf("Failed to fetch remote log level: %v", err)
				continue
			}

			// Send the new log level to the channel
			rl.logLevelChannel <- newLevel

		case newLevel := <-rl.logLevelChannel:
			rl.logger.level = newLevel

		default:
		}
	}
}

func (rl *RemoteLevelService) fetchLogLevel() (Level, error) {
	client := &http.Client{
		Timeout: 5 * time.Second, // Add timeout for request
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", rl.url, http.NoBody)
	if err != nil {
		return rl.LogLevel, err
	}

	req.Header.Set("Access-Key", rl.accessKey)
	req.Header.Set("App-Name", rl.appName)

	resp, err := client.Do(req)
	if err != nil {
		return rl.LogLevel, err
	}
	defer resp.Body.Close()

	type data struct {
		ServiceName string            `json:"serviceName"`
		Level       map[string]string `json:"logLevel"`
	}

	level := struct {
		Data []data `json:"data"`
	}{}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return rl.LogLevel, err
	}

	err = json.Unmarshal(body, &level)
	if err != nil {
		rl.logger.Errorf("Logging Service returned %v", err)
	}

	if len(level.Data) > 0 {
		logLevel := level.Data[0].Level["LOG_LEVEL"]
		newLevel := GetLevelFromString(logLevel)

		return newLevel, nil
	}

	return rl.LogLevel, nil
}
