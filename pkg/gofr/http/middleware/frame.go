package middleware

import (
	"net/http"
	"time"

	"gofr.dev/pkg/gofr/internal/quietspan"
)

// frameHandler is Tracer, Logging, CORS and Metrics, composed in that order, served from one stack
// frame. GoFr's server composes exactly that chain for every route, and each layer was a frame of
// its own, between the router and the handler, on every request.
//
// It runs each layer's steps in the order the composed layers ran them, with the same writer and
// request at each step, and defers the same calls in the same order: on return or panic the metric
// is recorded first, then the request is logged, then a panic is recovered, then the pooled writer
// is released, and the span ends last.
type frameHandler struct {
	logging *loggingLayer
	cors    *corsLayer
	metrics *metricsLayer
	inner   http.Handler
}

// fuseFramework returns the frame for Tracer(inner) when inner is Logging, CORS and Metrics
// composed in that order, and nil otherwise.
func fuseFramework(inner http.Handler) http.Handler {
	l, ok := inner.(*loggingHandler)
	if !ok {
		return nil
	}

	c, ok := l.inner.(*corsHandler)
	if !ok {
		return nil
	}

	m, ok := c.inner.(*metricsHandler)
	if !ok {
		return nil
	}

	return &frameHandler{logging: l.layer, cors: c.layer, metrics: m.layer, inner: m.inner}
}

// ClaimsQuietHost makes the frame a quietspan.Claimant, as Tracer's handler is.
func (*frameHandler) ClaimsQuietHost(quietspan.ClaimToken) {}

// ServeHTTP is tracerHandler, loggingHandler, corsHandler and metricsHandler's ServeHTTP, in turn.
func (f *frameHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Tracer.
	r, span, tsrw := startSpan(w, r)
	if span != nil {
		defer span.End()

		if tsrw != nil {
			w = tsrw

			defer setStatusAttribute(span, tsrw)
		}
	}

	// Logging.
	l := f.logging

	srw, rec, traceID, spanID := l.begin(w, r)

	defer l.release(srw)

	defer func() { panicRecovery(recover(), srw, l.sink.logger) }()

	// The rest has no early return: the compiler open-codes a function's defers only while its
	// returns times its defers stays at 15 or below, and otherwise records each defer at run time,
	// which cost more than the frames this saves.
	if isLogProbeDisabled(l.probes, r.URL.Path) {
		// Logging passes its own writer on, not srw.
		f.serveCORSAndMetrics(w, r)
	} else {
		defer rec.log(srw, r, time.Now(), traceID, spanID, l.sink)

		// CORS, then Metrics.
		if f.cors.serve(srw, r) {
			msrw, path, templated := metricsBegin(srw, r)
			if path != graphqlPath {
				defer f.metrics.record(msrw, r, path, templated, time.Now())
			}

			f.inner.ServeHTTP(msrw, r)
		}
	}
}

// serveCORSAndMetrics is the CORS and Metrics part of ServeHTTP for a probe path whose request is
// not logged. It is a frame of its own: the probe paths are not the request path being flattened.
func (f *frameHandler) serveCORSAndMetrics(w http.ResponseWriter, r *http.Request) {
	if !f.cors.serve(w, r) {
		return
	}

	msrw, path, templated := metricsBegin(w, r)
	if path == graphqlPath {
		f.inner.ServeHTTP(msrw, r)
		return
	}

	defer f.metrics.record(msrw, r, path, templated, time.Now())

	f.inner.ServeHTTP(msrw, r)
}
