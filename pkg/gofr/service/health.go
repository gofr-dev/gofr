package service

import (
	"context"
	"net/http"
	"time"
)

const (
	serviceUp      = "UP"
	serviceDown    = "DOWN"
	defaultTimeout = 5
)

type Health struct {
	Status  string                 `json:"status"`
	Details map[string]interface{} `json:"details"`
}

func (h *httpService) HealthCheck(ctx context.Context) *Health {
	return h.getHealthResponseForEndpoint(ctx, ".well-known/alive", defaultTimeout)
}

func (h *httpService) getHealthResponseForEndpoint(_ context.Context, endpoint string, timeout int) *Health {
	var healthResponse = Health{
		Details: make(map[string]interface{}),
	}

	// caeate a new context with 5 second timeout for healthCheck call.
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// send a new context as we can have downstream services taking too long
	// which may cancel the original health check http request
	resp, err := h.Get(ctx, endpoint, nil)

	if err != nil || resp == nil {
		healthResponse.Status = serviceDown
		healthResponse.Details["error"] = err.Error()

		return &healthResponse
	}

	defer resp.Body.Close()

	healthResponse.Details["host"] = resp.Request.URL.Host

	if resp.StatusCode == http.StatusOK {
		healthResponse.Status = serviceUp

		return &healthResponse
	}

	healthResponse.Status = serviceDown
	healthResponse.Details["error"] = "service down"

	return &healthResponse
}
