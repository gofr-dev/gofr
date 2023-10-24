package gofr

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/config"
)

const metricErr = errors.Error("invalid/duplicate metrics collector registration attempted")

// testCase represents a test case used in multiple test functions
// Need this type as same testcase is used for every test function
type testCase struct {
	desc string
	err  error
}

func initializeTest() ([]testCase, *Gofr) {
	testCases := []testCase{
		{"success-case:when err is not nil", nil},
		{"error-case:when duplicate metrics collector is registered", metricErr},
	}

	app := NewWithConfig(&config.MockConfig{Data: map[string]string{"APP_NAME": "gofr"}})

	return testCases, app
}

func Test_NewCounter(t *testing.T) {
	testCases, app := initializeTest()

	for i, tc := range testCases {
		err := app.NewCounter("new_counter", "New Counter", "id")

		assert.Equal(t, tc.err, err, "Test[%d] Failed:%v", i, tc.desc)
	}
}

func Test_NewHistogram(t *testing.T) {
	testCases, app := initializeTest()

	for i, tc := range testCases {
		err := app.NewHistogram("new_histogram", "New Histogram", []float64{.5, 1, 2}, "id")

		assert.Equal(t, tc.err, err, "TESTCASE[%d] NewHistogram:%v", i, tc.desc)
	}
}

func Test_NewSummary(t *testing.T) {
	testCases, app := initializeTest()

	for i, tc := range testCases {
		err := app.NewSummary("new_summary", "New Summary")

		assert.Equal(t, tc.err, err, "TESTCASE[%d] NewSummary:%v", i, tc.desc)
	}
}

func Test_NewGauge(t *testing.T) {
	testCases, app := initializeTest()

	for i, tc := range testCases {
		err := app.NewGauge("new_gauge", "New Gauge")

		assert.Equal(t, tc.err, err, "TESTCASE[%d] NewGauge:%v", i, tc.desc)
	}
}
