package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	serviceUp      = "UP"
	serviceDown    = "DOWN"
	defaultTimeout = 5

	// serviceDownReason is the "error" detail set whenever the check reaches the service and the
	// service answers with something other than 200. Named because the tests assert on it too.
	serviceDownReason = "service down"

	AlivePath  = "/.well-known/alive"
	HealthPath = "/.well-known/health"
)

// The body of a health response is read and discarded before it is closed, because net/http only
// returns a connection to the idle pool once its body has been read to EOF: closing it unread costs
// a new TCP (and TLS) connection on every check. The drain is bounded in size and time so it never
// changes the outcome of a check, only whether its connection is reused.
const (
	// maxHealthDrainBytes matches the bound net/http's server uses for the same purpose
	// (maxPostHandlerReadBytes). A larger body is closed unread, as before.
	maxHealthDrainBytes = 256 << 10

	// healthDrainTimeout bounds how long a body that stalls after its headers can hold the check.
	// The status is already known from the headers by then.
	healthDrainTimeout = 100 * time.Millisecond
)

type Health struct {
	Status  string         `json:"status"`
	Details map[string]any `json:"details"`
}

func (h *httpService) HealthCheck(ctx context.Context) *Health {
	return h.getHealthResponseForEndpoint(ctx, strings.TrimPrefix(AlivePath, "/"), defaultTimeout)
}

func (h *httpService) getHealthResponseForEndpoint(ctx context.Context, endpoint string, timeout int) *Health {
	var healthResponse = Health{
		Details: make(map[string]any),
	}

	// A new context for the health check call, detached from the caller's and carrying the check's
	// own timeout: we can have downstream services taking too long, and that must not cancel the
	// original health check http request.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Duration(timeout)*time.Second)
	defer cancel()

	// reqCtx exists so the drain below can be aborted. drainAndClose reads the body to EOF to give
	// the connection back to the pool, and arms a timer against this cancel, so a backend that
	// answers its status line promptly and then dribbles the body cannot hold the check open for
	// the whole timeout above.
	reqCtx, cancelReq := context.WithCancel(ctx)
	defer cancelReq()

	resp, err := h.Get(reqCtx, endpoint, nil)

	if err != nil || resp == nil {
		healthResponse.Status = serviceDown
		healthResponse.Details["error"] = err.Error()

		return &healthResponse
	}

	defer drainAndClose(resp, cancelReq)

	healthResponse.Details["host"] = resp.Request.URL.Host

	if resp.StatusCode == http.StatusOK {
		healthResponse.Status = serviceUp

		return &healthResponse
	}

	healthResponse.Status = serviceDown
	healthResponse.Details["error"] = serviceDownReason

	return &healthResponse
}

// drainAndClose reads up to maxHealthDrainBytes of resp.Body so that its connection can be reused,
// then closes it. abort cancels the request that produced resp; it is called if the drain has not
// finished within healthDrainTimeout, which ends the read and closes that one connection.
func drainAndClose(resp *http.Response, abort context.CancelFunc) {
	timer := time.AfterFunc(healthDrainTimeout, abort)

	// The +1 makes a body of exactly maxHealthDrainBytes read on to EOF, which is what marks its
	// connection reusable. The error is irrelevant: the drain only decides whether the connection
	// is reused, never the result of the check.
	_, _ = io.CopyN(io.Discard, resp.Body, maxHealthDrainBytes+1)

	timer.Stop()

	_ = resp.Body.Close()
}
