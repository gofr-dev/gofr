package service

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

// 1. HTTP client should not bombard the downstream service
// 2. Use a constant time period to achieve this (for ex: 5 seconds)
// 3. Let's say the HTTP client makes a request to `xyz-service/abc`, make a request to ensure the service is up
// 4. If the service is down, return 500, and asynchronously make requests after `n` seconds, where `n` is the constant
//    time period (for ex: 5 seconds)

type surgeProtector struct {
	// customHeartbeatURL is the URL that the surge protector will asynchronously call to figure out the status of a
	// service, default value is `/.well-known/heartbeat`
	customHeartbeatURL string

	// retryFrequency is the retry frequency (in seconds) of the asynchronous job that routinely checks the status
	// of services that are down
	retryFrequencySeconds int

	once sync.Once

	mu sync.Mutex

	// isEnabled is true if the surge protector is enabled
	isEnabled bool

	logger log.Logger
}

// checkHealth performs a health check and returns the health status for the surgeProtector
func (sp *surgeProtector) checkHealth(url string, ch chan bool) {
	for {
		var isHealthy bool

		sp.mu.Lock()

		// this will be used to log an error when the health-check/heartbeat fails
		var err errors.HealthCheckFailed

		err.Dependency = url

		resp, getErr := http.Get(url + sp.customHeartbeatURL)
		if getErr != nil {
			err.Err = getErr
		} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			err.Reason = "Status Code " + strconv.Itoa(resp.StatusCode)
			resp.Body.Close()
		} else {
			isHealthy = true
			resp.Body.Close()
		}

		if !isHealthy {
			sp.logger.Errorf("%v", err)
		}

		retryFrequency := sp.retryFrequencySeconds

		sp.mu.Unlock()

		ch <- isHealthy

		time.Sleep(time.Duration(retryFrequency) * time.Second)
	}
}
