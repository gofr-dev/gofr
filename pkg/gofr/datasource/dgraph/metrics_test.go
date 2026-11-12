package dgraph

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestPrometheusMetrics(t *testing.T) {
	tests := []struct {
		desc        string
		record      string
		expObserved int
	}{
		{desc: "records to registered histogram", record: "dgraph_test_registered_duration", expObserved: 1},
		{desc: "ignores unknown histogram", record: "dgraph_test_unknown_duration", expObserved: 0},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			const name = "dgraph_test_registered_duration"

			p := &PrometheusMetrics{histograms: map[string]*prometheus.HistogramVec{}}

			p.NewHistogram(name, "test histogram", 1, 10, 100)
			t.Cleanup(func() { prometheus.Unregister(p.histograms[name]) })

			p.RecordHistogram(t.Context(), tc.record, 5)

			require.Contains(t, p.histograms, name)

			// Each observed label set yields one collected metric.
			collected := make(chan prometheus.Metric, 10)
			p.histograms[name].Collect(collected)
			close(collected)

			require.Len(t, collected, tc.expObserved)
		})
	}
}
