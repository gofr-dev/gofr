package service

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

// TestRetryConfig_ZeroValueFieldsKeepTodaysRule pins that setting one field leaves the others at
// today's behavior: a nil RetryOn still retries any status above 500 and not a 429, and nil Methods
// still retries a POST.
func TestRetryConfig_ZeroValueFieldsKeepTodaysRule(t *testing.T) {
	configs := []struct {
		desc string
		cfg  RetryConfig
	}{
		{desc: "only Budget", cfg: RetryConfig{MaxRetries: 2, Budget: &RetryBudget{}}},
		{desc: "only Backoff", cfg: RetryConfig{MaxRetries: 2, Backoff: ConstantBackoff{Delay: time.Millisecond}}},
	}

	get := func(ctx context.Context, s HTTP) (*http.Response, error) { return s.Get(ctx, "test", nil) }
	post := func(ctx context.Context, s HTTP) (*http.Response, error) { return s.Post(ctx, "test", nil, nil) }
	patch := func(ctx context.Context, s HTTP) (*http.Response, error) { return s.Patch(ctx, "test", nil, nil) }

	calls := []struct {
		desc     string
		call     func(context.Context, HTTP) (*http.Response, error)
		status   int
		wantHits int32
	}{
		{desc: "POST 503 is retried", call: post, status: http.StatusServiceUnavailable, wantHits: 3},
		{desc: "PATCH 503 is retried", call: patch, status: http.StatusServiceUnavailable, wantHits: 3},
		{desc: "GET 501 is retried", call: get, status: http.StatusNotImplemented, wantHits: 3},
		{desc: "GET 500 is not retried", call: get, status: http.StatusInternalServerError, wantHits: 1},
		{desc: "GET 429 is not retried", call: get, status: http.StatusTooManyRequests, wantHits: 1},
	}

	for _, c := range configs {
		for i, tc := range calls {
			t.Run(c.desc+"/"+tc.desc, func(t *testing.T) {
				server, hits := statusSequence(t, nil, tc.status)

				cfg := c.cfg
				svc, _ := newPolicyService(t, server.URL, nil, &cfg)

				resp, err := tc.call(t.Context(), svc)

				require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Equal(t, fmt.Sprintf("hit %d", tc.wantHits), readBody(t, resp), "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Equal(t, tc.wantHits, hits.Load(), "TEST[%d], Failed.\n%s", i, tc.desc)
			})
		}
	}
}

// TestRetryConfig_ConfiguredSwitch pins the one switch: a RetryConfig with only MaxRetries keeps today's
// metric shape, retries an open circuit breaker and ignores the caller's deadline; one that sets any
// other field does none of those.
func TestRetryConfig_ConfiguredSwitch(t *testing.T) {
	tests := []struct {
		desc           string
		cfg            RetryConfig
		wantConfigured bool
	}{
		{desc: "only MaxRetries", cfg: RetryConfig{MaxRetries: 1}},
		{desc: "MaxRetries and Budget", cfg: RetryConfig{MaxRetries: 1, Budget: &RetryBudget{}}, wantConfigured: true},
		{desc: "MaxRetries and Backoff", cfg: RetryConfig{MaxRetries: 1, Backoff: ConstantBackoff{}}, wantConfigured: true},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// Metrics: one retry of a 503, counted in the shape of the mode.
			ctrl := gomock.NewController(t)
			metrics := NewMockMetrics(ctrl)

			metrics.EXPECT().RecordHistogram(gomock.Any(), "app_http_service_response", gomock.Any(),
				gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				gomock.Any(), gomock.Any()).AnyTimes()

			if tc.wantConfigured {
				metrics.EXPECT().IncrementCounter(gomock.Any(), retryCountMetric, "service", "svc", "reason", retryReasonStatus5xx)
				metrics.EXPECT().RecordHistogram(gomock.Any(), retryWaitMetric, gomock.Any(), "service", "svc")
			} else {
				metrics.EXPECT().IncrementCounter(gomock.Any(), retryCountMetric, "service", "svc")
			}

			server, _ := statusSequence(t, nil, http.StatusServiceUnavailable)

			cfg := tc.cfg
			svc, _ := newPolicyService(t, server.URL, metrics, &cfg)

			resp, err := svc.Get(t.Context(), "test", nil)
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, "hit 2", readBody(t, resp), "TEST[%d], Failed.\n%s", i, tc.desc)

			// Open circuit breaker: retried only by a plain RetryConfig.
			probe := &attemptProbe{fail: func(int32) error { return ErrCircuitOpen }}
			cfg = tc.cfg
			resp, err = NewHTTPService(server.URL, logging.NewMockLogger(logging.ERROR), nil, probe, &cfg).Get(t.Context(), "test", nil)
			requireNoResponse(t, resp)
			require.ErrorIs(t, err, ErrCircuitOpen, "TEST[%d], Failed.\n%s", i, tc.desc)

			wantAttempts := int32(2)
			if tc.wantConfigured {
				wantAttempts = 1
			}

			assert.Equal(t, wantAttempts, probe.attempts.Load(), "TEST[%d], Failed.\n%s", i, tc.desc)

			// Deadline room: an hour's wait does not fit a minute's deadline, unless the check is off.
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()

			cfg = tc.cfg
			rp, ok := NewHTTPService(server.URL, logging.NewMockLogger(logging.ERROR), nil, &cfg).(*retryProvider)
			require.True(t, ok, "TEST[%d], RetryConfig must wrap the service in a retryProvider", i)
			assert.Equal(t, !tc.wantConfigured, rp.roomForRetry(ctx, time.Hour), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

// TestRetryConfig_ConfiguredByAnyField pins that every field besides MaxRetries, set on its own, turns the
// configured behaviors on -- except MaxRetryAfter and IdempotencyKeyHeader, which only qualify
// RespectRetryAfter and Methods and do nothing alone.
func TestRetryConfig_ConfiguredByAnyField(t *testing.T) {
	tests := []struct {
		desc           string
		cfg            RetryConfig
		wantConfigured bool
	}{
		{desc: "zero value", cfg: RetryConfig{}},
		{desc: "MaxRetries", cfg: RetryConfig{MaxRetries: 3}},
		{desc: "negative MaxRetries", cfg: RetryConfig{MaxRetries: -1}},
		{desc: "Backoff", cfg: RetryConfig{Backoff: ConstantBackoff{}}, wantConfigured: true},
		{desc: "RetryOn", cfg: RetryConfig{RetryOn: StandardRetryable}, wantConfigured: true},
		{desc: "RespectRetryAfter", cfg: RetryConfig{RespectRetryAfter: true}, wantConfigured: true},
		{desc: "MaxRetryAfter alone", cfg: RetryConfig{MaxRetries: 3, MaxRetryAfter: time.Second}},
		{desc: "Methods", cfg: RetryConfig{Methods: []string{http.MethodGet}}, wantConfigured: true},
		{desc: "empty non-nil Methods", cfg: RetryConfig{Methods: []string{}}, wantConfigured: true},
		{desc: "IdempotencyKeyHeader alone", cfg: RetryConfig{MaxRetries: 3, IdempotencyKeyHeader: "X-Key"}},
		{desc: "PerAttemptTimeout", cfg: RetryConfig{PerAttemptTimeout: time.Second}, wantConfigured: true},
		{desc: "Budget", cfg: RetryConfig{Budget: &RetryBudget{}}, wantConfigured: true},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, !tc.wantConfigured, tc.cfg.resolve().legacy, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
