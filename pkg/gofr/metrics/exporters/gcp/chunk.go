package gcp

import (
	"context"
	"errors"
	"time"

	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// maxPointsPerRequest is the most points the Telemetry API accepts in one
// request. Past it the API rejects the whole request, not just the excess:
//
//	rpc error: code = InvalidArgument desc =
//	A maximum of 200 points can be written in a single request.
//
// Cumulative temporality re-sends every attribute set a process has seen, so a
// long-lived process grows into this limit with uptime and, unsplit, would lose
// every collection from then on.
const maxPointsPerRequest = 200

// newReader returns the periodic reader the exporter hands to the SDK, with exp
// wrapped so no request exceeds maxPointsPerRequest.
func newReader(exp metricSdk.Exporter, interval time.Duration) metricSdk.Reader {
	return metricSdk.NewPeriodicReader(
		&chunkingExporter{Exporter: exp, limit: maxPointsPerRequest},
		metricSdk.WithInterval(interval),
	)
}

// chunkingExporter sends each collection as a sequence of requests of at most
// limit points. Temporality, aggregation, flush and shutdown pass through.
//
// The OTel SDK has an equivalent batcher, but only behind the experimental
// OTEL_GO_X_METRIC_EXPORT_BATCH_SIZE environment variable; this makes the
// limit a property of the exporter instead of something every deployment has to
// discover. When both are active the SDK's batches arrive already small enough
// and pass through unchanged.
type chunkingExporter struct {
	metricSdk.Exporter
	limit int
}

// Export sends every chunk even when an earlier one fails, so one rejected
// request costs only its own points. It stops once ctx is done, because every
// remaining request would fail the same way.
func (e *chunkingExporter) Export(ctx context.Context, rm *metricdata.ResourceMetrics) error {
	var errs []error

	for _, chunk := range splitResourceMetrics(rm, e.limit) {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}

		if err := e.Exporter.Export(ctx, chunk); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// splitResourceMetrics splits rm into ResourceMetrics of at most limit points
// each, keeping the points' order, their scope and their metric's metadata. A
// metric too large for the space left in a chunk is itself split across chunks.
// rm is returned as is when it already fits or when limit is not positive; it is
// never modified.
//
// A metric with no data points carries nothing to write and is not carried into
// a split.
func splitResourceMetrics(rm *metricdata.ResourceMetrics, limit int) []*metricdata.ResourceMetrics {
	if limit <= 0 || resourcePoints(rm) <= limit {
		return []*metricdata.ResourceMetrics{rm}
	}

	s := splitter{limit: limit, src: rm}

	for i, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			total := metricPoints(m)

			for lo := 0; lo < total; {
				hi := lo + min(total-lo, s.room())
				s.add(i, sliceMetric(m, lo, hi), hi-lo)
				lo = hi
			}
		}
	}

	return s.out
}

// splitter accumulates the chunks of one split.
type splitter struct {
	limit int
	src   *metricdata.ResourceMetrics
	out   []*metricdata.ResourceMetrics
	// points in the last chunk, and the index in src of its last scope.
	points    int
	lastScope int
}

// room is how many points the current chunk can still take, starting a new one
// when it is full.
func (s *splitter) room() int {
	if len(s.out) == 0 || s.points == s.limit {
		s.out = append(s.out, &metricdata.ResourceMetrics{Resource: s.src.Resource})
		s.points, s.lastScope = 0, -1
	}

	return s.limit - s.points
}

// add appends m, taken from src.ScopeMetrics[scope], to the current chunk.
func (s *splitter) add(scope int, m metricdata.Metrics, n int) {
	chunk := s.out[len(s.out)-1]

	if scope != s.lastScope {
		chunk.ScopeMetrics = append(chunk.ScopeMetrics, metricdata.ScopeMetrics{Scope: s.src.ScopeMetrics[scope].Scope})
		s.lastScope = scope
	}

	last := &chunk.ScopeMetrics[len(chunk.ScopeMetrics)-1]
	last.Metrics = append(last.Metrics, m)
	s.points += n
}

func resourcePoints(rm *metricdata.ResourceMetrics) int {
	n := 0

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			n += metricPoints(m)
		}
	}

	return n
}

// metricPoints counts m's data points. Aggregation is sealed by the SDK, so
// these are all the kinds there are; anything else is counted as one point and
// kept whole by sliceMetric.
func metricPoints(m metricdata.Metrics) int {
	switch d := m.Data.(type) {
	case metricdata.Gauge[int64]:
		return len(d.DataPoints)
	case metricdata.Gauge[float64]:
		return len(d.DataPoints)
	case metricdata.Sum[int64]:
		return len(d.DataPoints)
	case metricdata.Sum[float64]:
		return len(d.DataPoints)
	case metricdata.Histogram[int64]:
		return len(d.DataPoints)
	case metricdata.Histogram[float64]:
		return len(d.DataPoints)
	case metricdata.ExponentialHistogram[int64]:
		return len(d.DataPoints)
	case metricdata.ExponentialHistogram[float64]:
		return len(d.DataPoints)
	case metricdata.Summary:
		return len(d.DataPoints)
	default:
		return 1
	}
}

// sliceMetric returns m carrying only its data points [lo, hi). The slices share
// rm's backing arrays but are capped at hi, so nothing appended downstream can
// overwrite a point that belongs to the next chunk.
func sliceMetric(m metricdata.Metrics, lo, hi int) metricdata.Metrics {
	switch d := m.Data.(type) {
	case metricdata.Gauge[int64]:
		d.DataPoints = d.DataPoints[lo:hi:hi]
		m.Data = d
	case metricdata.Gauge[float64]:
		d.DataPoints = d.DataPoints[lo:hi:hi]
		m.Data = d
	case metricdata.Sum[int64]:
		d.DataPoints = d.DataPoints[lo:hi:hi]
		m.Data = d
	case metricdata.Sum[float64]:
		d.DataPoints = d.DataPoints[lo:hi:hi]
		m.Data = d
	case metricdata.Histogram[int64]:
		d.DataPoints = d.DataPoints[lo:hi:hi]
		m.Data = d
	case metricdata.Histogram[float64]:
		d.DataPoints = d.DataPoints[lo:hi:hi]
		m.Data = d
	case metricdata.ExponentialHistogram[int64]:
		d.DataPoints = d.DataPoints[lo:hi:hi]
		m.Data = d
	case metricdata.ExponentialHistogram[float64]:
		d.DataPoints = d.DataPoints[lo:hi:hi]
		m.Data = d
	case metricdata.Summary:
		d.DataPoints = d.DataPoints[lo:hi:hi]
		m.Data = d
	}

	return m
}
