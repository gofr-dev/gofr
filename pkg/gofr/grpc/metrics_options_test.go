package grpc

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"gofr.dev/pkg/gofr/logging"
	gofrmetrics "gofr.dev/pkg/gofr/metrics"
	"gofr.dev/pkg/gofr/metrics/exporters"
)

type optionMetrics struct {
	Metrics
	ctx   context.Context
	name  string
	value float64
	opts  []metric.RecordOption
}

func (m *optionMetrics) RecordHistogramOpt(ctx context.Context, name string, value float64, opts ...metric.RecordOption) {
	m.ctx, m.name, m.value, m.opts = ctx, name, value, opts
}

func TestRPCMetricOptions(t *testing.T) {
	ctx := t.Context()

	recorder := &optionMetrics{}
	for _, stream := range []string{"", "SERVER_STREAM"} {
		recordGRPCMetrics(ctx, recorder, "rpc", time.Millisecond, "/test/Method", stream, 4)
		assert.Equal(t, ctx, recorder.ctx)
		assert.Equal(t, "rpc", recorder.name)
		assert.InDelta(t, float64(1), recorder.value, 0)
		attrs := metric.NewRecordConfig(recorder.opts).Attributes()
		value, ok := attrs.Value(attribute.Key("status"))
		require.True(t, ok)
		assert.Equal(t, "4", value.AsString())
		value, ok = attrs.Value(attribute.Key("method"))
		require.True(t, ok)
		assert.Equal(t, "/test/Method", value.AsString())
		value, ok = attrs.Value(attribute.Key("stream_type"))
		assert.Equal(t, stream != "", ok)

		if ok {
			assert.Equal(t, stream, value.AsString())
		}

		previous := recorder.opts
		recordGRPCMetrics(ctx, recorder, "rpc", time.Millisecond, "/test/Method", stream, 4)
		assert.Same(t, &previous[0], &recorder.opts[0])
	}
}

func TestRPCOptionCacheBound(t *testing.T) {
	var (
		cache   rpcOptionCache
		workers sync.WaitGroup
	)
	for worker := range 4 {
		workers.Go(func() {
			for i := range maxRPCMetricOptions {
				key := rpcMetricKey{method: fmt.Sprintf("/health/%d/%d", worker, i), code: 5}
				opts := cache.get(key)
				attrs := metric.NewRecordConfig(opts).Attributes()
				value, ok := attrs.Value(attribute.Key("method"))
				assert.True(t, ok)
				assert.Equal(t, key.method, value.AsString())
			}
		})
	}

	workers.Wait()
	assert.Len(t, cache.options, maxRPCMetricOptions)
}

type legacyMetrics struct{ Metrics }

func BenchmarkRPCMetricsOptions(b *testing.B) {
	cfg := exporters.Config{AppName: "grpc-bench", AppVersion: "test"}
	logger := logging.NewMockLogger(logging.ERROR)
	shutdown, meter := exporters.Build(b.Context(), &cfg, logger)
	b.Cleanup(func() {
		if shutdown != nil {
			_ = shutdown(context.Background())
		}
	})

	manager := gofrmetrics.NewMetricsManager(meter, logger)
	manager.NewHistogram("rpc", "RPC duration")

	for _, tc := range []struct {
		name     string
		recorder Metrics
	}{
		{name: "cached", recorder: manager},
		{name: "labels", recorder: legacyMetrics{Metrics: manager}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			ctx := b.Context()
			recordGRPCMetrics(ctx, tc.recorder, "rpc", time.Millisecond, "/test/Method", "", 0)
			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				recordGRPCMetrics(ctx, tc.recorder, "rpc", time.Millisecond, "/test/Method", "", 0)
			}
		})
	}
}
