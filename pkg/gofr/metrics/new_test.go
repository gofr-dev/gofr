package metrics

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func Test_New(t *testing.T) {
	metric := NewMetric()

	mockMetric := NewMockMetric(gomock.NewController(t))

	testcases := []struct {
		metric Metric
		desc   string
		err    error
	}{
		{metric, "success-case", nil},
		{metric, "error-case", metricErr},
		{mockMetric, "error-case", errInvalidType},
		{nil, "error-case", errInvalidMetric},
	}

	for i, tc := range testcases {
		err := NewCounter(tc.metric, "new_counter", "New Counter", "id")
		assert.Equal(t, tc.err, err, "TESTCASE[%v] NewCounter", i)

		err = NewHistogram(tc.metric, "new_histogram", "New Histogram", []float64{.5, 1, 2}, "id")
		assert.Equal(t, tc.err, err, "TESTCASE[%v] NewHistogram", i)

		err = NewGauge(tc.metric, "new_gauge", "New Gauge")
		assert.Equal(t, tc.err, err, "TESTCASE[%v] NewGauge", i)

		err = NewSummary(tc.metric, "new_summary", "New Summary")
		assert.Equal(t, tc.err, err, "TESTCASE[%v] NewSummary", i)
	}
}
