package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cardRecorder captures the label sets the middleware actually records, so these
// tests observe the wiring rather than the helper it calls.
type cardRecorder struct{ labels [][]string }

func (c *cardRecorder) RecordHistogram(_ context.Context, _ string, _ float64, labels ...string) {
	c.labels = append(c.labels, labels)
}

func (*cardRecorder) IncrementCounter(context.Context, string, ...string)            {}
func (*cardRecorder) DeltaUpDownCounter(context.Context, string, float64, ...string) {}
func (*cardRecorder) SetGauge(string, float64, ...string)                            {}

// series returns the distinct (path, method) pairs recorded -- one metric series
// each, and one retained datapoint each.
func (c *cardRecorder) series() map[[2]string]int {
	out := map[[2]string]int{}

	for _, l := range c.labels {
		var pair [2]string

		for i := 0; i+1 < len(l); i += 2 {
			switch l[i] {
			case "path":
				pair[0] = l[i+1]
			case "method":
				pair[1] = l[i+1]
			}
		}

		out[pair]++
	}

	return out
}

// cardServe drives the real Metrics middleware, with no router, so every request
// is untemplated -- the state an unmatched request reaches this middleware in.
func cardServe(t *testing.T, rec *cardRecorder, method, path string) {
	t.Helper()

	h := Metrics(rec)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequestWithContext(context.Background(), method, path, http.NoBody)
	h.ServeHTTP(httptest.NewRecorder(), req)
}

// TestUntemplatedSeriesAreBounded is what this change exists for.
//
// A histogram retains a datapoint per distinct attribute set for the life of the
// process, and the meter provider caps how many it will hold. Series past that
// cap fold into one overflow bucket, so caller-controlled labels arriving first
// occupy the slots and the real routes registered later are the ones that
// disappear from the dashboard. This asserts the caller-controlled population is
// finite, measured through the middleware rather than through the helper.
func TestUntemplatedSeriesAreBounded(t *testing.T) {
	untemplatedLabels.reset()
	defer untemplatedLabels.reset()

	rec := &cardRecorder{}

	for i := range untemplatedLabelLimit * 3 {
		cardServe(t, rec, http.MethodGet, fmt.Sprintf("/no/such/path/%d", i))
	}

	assert.LessOrEqual(t, len(rec.series()), untemplatedLabelLimit+1,
		"%d invented paths produced %d metric series", untemplatedLabelLimit*3, len(rec.series()))
}

// TestInventedMethodsAreBounded closes the other half of the key. Holding the
// path fixed and varying the method reaches the same datapoint table, and would
// never consult a path-only ceiling.
func TestInventedMethodsAreBounded(t *testing.T) {
	untemplatedLabels.reset()
	defer untemplatedLabels.reset()

	rec := &cardRecorder{}

	for i := range untemplatedLabelLimit * 3 {
		cardServe(t, rec, fmt.Sprintf("M%05d", i), "/fixed")
	}

	assert.LessOrEqual(t, len(rec.series()), untemplatedLabelLimit+1,
		"%d invented methods produced %d metric series", untemplatedLabelLimit*3, len(rec.series()))
}

// TestTemplatedRoutesSurviveJunkTraffic pins the point of bounding at all: a real
// route must still get its own series while junk is holding the ceiling down.
func TestTemplatedRoutesSurviveJunkTraffic(t *testing.T) {
	untemplatedLabels.reset()
	defer untemplatedLabels.reset()

	junk := &cardRecorder{}
	for i := range untemplatedLabelLimit + 50 {
		cardServe(t, junk, http.MethodGet, fmt.Sprintf("/junk/%d", i))
	}

	rec := &cardRecorder{}
	router := mux.NewRouter()
	router.Use(Metrics(rec))
	router.NewRoute().Methods(http.MethodGet).Path("/users/{id}").
		Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	router.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/users/42", http.NoBody))

	require.Len(t, rec.labels, 1)
	assert.Contains(t, rec.series(), [2]string{"/users/{id}", http.MethodGet},
		"a templated route keeps its own series regardless of junk traffic")
}

func TestUntemplatedBudget(t *testing.T) {
	tests := []struct {
		ceiling  int
		expected int64
	}{
		{ceiling: 2000, expected: untemplatedLabelLimit},
		{ceiling: 100000, expected: untemplatedLabelLimit},
		{ceiling: 100, expected: 25},
		{ceiling: 3, expected: 0},
		{ceiling: 0, expected: untemplatedLabelLimit},
		{ceiling: -1, expected: untemplatedLabelLimit},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, untemplatedBudget(tt.ceiling), "ceiling %d", tt.ceiling)
	}
}

// TestLowCeilingKeepsRoutesReachable is the case a fixed budget got wrong: with
// the provider ceiling at 100, 512 caller-controlled pairs alone would fill it.
// Through WithCardinalityLimit the junk takes at most a quarter and a real route
// still records under its own template.
func TestLowCeilingKeepsRoutesReachable(t *testing.T) {
	untemplatedLabels.reset()
	defer untemplatedLabels.reset()

	const ceiling = 100

	junk := &cardRecorder{}
	h := Metrics(junk, WithCardinalityLimit(ceiling))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	for i := range ceiling * 3 {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(),
			http.MethodGet, fmt.Sprintf("/.env.scan-%d", i), http.NoBody))
	}

	assert.LessOrEqual(t, len(junk.series()), ceiling/untemplatedCeilingShare+1,
		"junk may take at most a quarter of the ceiling, plus the collapsed series")

	rec := &cardRecorder{}
	router := mux.NewRouter()
	router.Use(Metrics(rec, WithCardinalityLimit(ceiling)))
	router.NewRoute().Methods(http.MethodGet).Path("/users/{id}").
		Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	router.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/users/7", http.NoBody))

	assert.Contains(t, rec.series(), [2]string{"/users/{id}", http.MethodGet})
}

// BenchmarkMetricLabels isolates the per-request cost this change adds in front
// of the recorder, for the three paths a request can take through it.
func BenchmarkMetricLabels(b *testing.B) {
	b.Run("templated", func(b *testing.B) {
		b.ReportAllocs()

		for range b.N {
			metricLabels("/users/{id}", http.MethodGet, true, untemplatedLabelLimit)
		}
	})

	b.Run("untemplated_admitted", func(b *testing.B) {
		untemplatedLabels.reset()
		b.Cleanup(untemplatedLabels.reset)
		metricLabels("/.env", http.MethodGet, false, untemplatedLabelLimit)
		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			metricLabels("/.env", http.MethodGet, false, untemplatedLabelLimit)
		}
	})

	b.Run("untemplated_collapsed", func(b *testing.B) {
		untemplatedLabels.reset()
		b.Cleanup(untemplatedLabels.reset)

		for i := range untemplatedLabelLimit {
			metricLabels(fmt.Sprintf("/fill/%d", i), http.MethodGet, false, untemplatedLabelLimit)
		}

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			metricLabels("/.env.new", http.MethodGet, false, untemplatedLabelLimit)
		}
	})
}
