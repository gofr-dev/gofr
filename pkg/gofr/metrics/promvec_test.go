package metrics

import (
	"net/http"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func Test_IncCounter(t *testing.T) {
	tcs := []struct {
		desc string
		name string
		err  error
	}{
		{"success-case", "test", nil},
		{"error-case", "test1", metricNotFound},
	}

	p := newPromVec()

	err := p.registerCounter("test", "testing method", "code", "method")
	if err != nil {
		t.Errorf("error while creating counter")
	}

	for i, tc := range tcs {
		err = p.IncCounter(tc.name, "200", http.MethodPost)

		assert.Equal(t, tc.err, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

//nolint:dupl // duplicate code is for different metrics
func Test_AddCounter(t *testing.T) {
	tcs := []struct {
		desc string
		name string
		err  error
	}{
		{"success-case", "test_counter", nil},
		{"error-case", "test1", metricNotFound},
	}

	p := newPromVec()

	err := p.registerCounter("test_counter", "testing method", "code", "method")
	if err != nil {
		t.Errorf("error while creating counter")
	}

	for i, tc := range tcs {
		err = p.AddCounter(tc.name, float64(i), "200", http.MethodPost)

		assert.Equal(t, tc.err, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_ObserveHistogram(t *testing.T) {
	tcs := []struct {
		desc string
		name string
		err  error
	}{
		{"success-case", "test_histogram", nil},
		{"error-case", "test", metricNotFound},
	}

	p := newPromVec()

	err := p.registerHistogram("test_histogram", "testing method",
		[]float64{.001, .003, .005, .01, .025, .05, .1, .2, .3, .4, .5, .75, 1, 2, 3, 5, 10, 30},
		"code", "method")
	if err != nil {
		t.Errorf("error while creating histogram")
	}

	for i, tc := range tcs {
		err = p.ObserveHistogram(tc.name, float64(i), "200", http.MethodPost)

		assert.Equal(t, tc.err, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_SetGauge(t *testing.T) {
	tcs := []struct {
		desc string
		name string
		err  error
	}{
		{"success-case", "test_gauge", nil},
		{"error-case", "test", metricNotFound},
	}

	p := newPromVec()

	err := p.registerGauge("test_gauge", "set value of gauge", "no_of_go_routines")
	if err != nil {
		t.Errorf("error while creating gauge")
	}

	for i, tc := range tcs {
		err = p.SetGauge(tc.name, float64(i), "no_of_go_routines")

		assert.Equal(t, tc.err, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

//nolint:dupl // duplicate code is for different metrics
func Test_ObserveSummary(t *testing.T) {
	tcs := []struct {
		desc string
		name string
		err  error
	}{
		{"success-case", "test_summary", nil},
		{"error-case", "test", metricNotFound},
	}

	p := newPromVec()

	err := p.registerSummary("test_summary", "testing method", "code", "method")
	if err != nil {
		t.Errorf("error while creating summary")
	}

	for i, tc := range tcs {
		err = p.ObserveSummary(tc.name, float64(i), "200", http.MethodPost)

		assert.Equal(t, tc.err, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_InvalidLabelError(t *testing.T) {
	p := newPromVec()

	err := p.registerCounter("label_counter", "testing method", "code", "method")
	if err != nil {
		t.Errorf("error while creating counter")
	}

	err = p.registerHistogram("label_histogram", "testing method",
		[]float64{.001, .003, .005, .01, .025, .05, .1, .2, .3, .4, .5, .75, 1, 2, 3, 5, 10, 30},
		"code", "method")
	if err != nil {
		t.Errorf("error while creating histogram")
	}

	err = p.registerGauge("label_gauge", "set value of gauge", "no_of_go_routines")
	if err != nil {
		t.Errorf("error while creating gauge")
	}

	err = p.registerSummary("label_summary", "testing method", "code", "method")
	if err != nil {
		t.Errorf("error while creating summary")
	}

	err = p.IncCounter("label_counter")
	handleError(t, err)

	err = p.AddCounter("label_counter", 2)
	handleError(t, err)

	err = p.SetGauge("label_gauge", 2)
	handleError(t, err)

	err = p.ObserveSummary("label_summary", 2)
	handleError(t, err)

	err = p.ObserveHistogram("label_histogram", 2)
	handleError(t, err)
}

func handleError(t *testing.T, err error) {
	errStr := "inconsistent label cardinality"
	if err == nil || !strings.Contains(err.Error(), errStr) {
		t.Errorf("expected err string %v, got %v", errStr, err)
	}
}

func TestErrorCase(t *testing.T) {
	expError := metricErr

	p := promVec{
		summary: make(map[string]*prometheus.SummaryVec),
	}

	errorRegisterSummary := p.registerSummary("test_counter", "testing method", "code", "method")
	assert.Equal(t, expError, errorRegisterSummary)

	errorRegisterCounter := p.registerCounter("test_counter", "testing method", "code", "method")
	assert.Equal(t, expError, errorRegisterCounter)

	errorRegisterHistogram := p.registerHistogram("test_counter", "testing method",
		[]float64{.001, .003, .005, .01, .025, .05, .1, .2, .3, .4, .5, .75, 1, 2, 3, 5, 10, 30}, "method")
	assert.Equal(t, expError, errorRegisterHistogram)

	errorRegisterGauge := p.registerGauge("test_counter", "testing method", "code", "method")
	assert.Equal(t, expError, errorRegisterGauge)
}
