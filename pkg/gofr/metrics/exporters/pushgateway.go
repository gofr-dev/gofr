package exporters

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"

	"gofr.dev/pkg/gofr/logging"
)

var errUnexpectedStatus = errors.New("unexpected status from Pushgateway")

// PushGateway pushes metrics from a Prometheus registry to a Pushgateway
// using a read-modify-write strategy to accumulate counters and histograms
// across short-lived CLI process runs.
//
// Why read-modify-write instead of a drop-in aggregation gateway?
//
// Short-lived CLI processes start counters at 0 on every run. A plain
// Pushgateway overwrites on each push, so counters never accumulate
// (run1=1, run2=1, run3=1 instead of 1,2,3). We evaluated two
// aggregation gateways that solve this server-side:
//
//   - Zapier prom-aggregation-gateway: actively maintained, but sums ALL
//     metric types including gauges. Our last-success-timestamp gauge
//     would produce nonsensical values (timestamp1 + timestamp2).
//   - Prometheus Gravel Gateway: supports per-metric-type aggregation via
//     a clearmode label (sum counters, replace gauges), which is exactly
//     what we need. However, the project has been dormant since Nov 2023,
//     has a single maintainer, no prebuilt Docker images, and only 117
//     stars — not a safe dependency for a framework.
//
// Read-modify-write on the standard Pushgateway gives us the right
// semantics (sum counters/histograms, replace gauges) with zero
// additional infrastructure and no dependency on niche projects.
// The trade-off is a small race window when concurrent CLI runs overlap,
// but this is unlikely for CLI workloads and the worst case is one lost
// increment.
type PushGateway struct {
	pushURL    string
	metricsURL string
	gatherer   prometheus.Gatherer
	logger     logging.Logger
	client     *http.Client
}

// NewPushGateway creates a PushGateway pusher for the given URL and job name.
// The gatherer controls which metrics are pushed — use a dedicated app registry
// (from NewAppRegistry) to avoid pushing Go runtime metrics, or
// prometheus.DefaultGatherer to push everything.
func NewPushGateway(url, jobName string, gatherer prometheus.Gatherer, l logging.Logger) *PushGateway {
	base := strings.TrimRight(url, "/")

	return &PushGateway{
		pushURL:    base + "/metrics/job/" + jobName,
		metricsURL: base + "/metrics",
		gatherer:   gatherer,
		logger:     l,
		client:     &http.Client{},
	}
}

// Push gathers local metrics, fetches existing metrics from the Pushgateway,
// merges them (summing counters/histograms, replacing gauges), and PUTs the
// result back. Should be called on shutdown.
func (p *PushGateway) Push(ctx context.Context) error {
	p.logger.Logf("Pushing metrics to Pushgateway")

	localFamilies, err := p.gatherer.Gather()
	if err != nil {
		p.logger.Errorf("failed to gather local metrics: %v", err)
		return err
	}

	existing := p.fetchExisting(ctx)

	merged := mergeMetrics(existing, localFamilies)

	if err := p.put(ctx, merged); err != nil {
		p.logger.Errorf("failed to push metrics to Pushgateway: %v", err)
		return err
	}

	return nil
}

// fetchExisting GETs /metrics from the Pushgateway and parses the response
// into a map of metric families. Returns an empty map on any failure so that
// Push can proceed with local-only values (first run or Pushgateway down).
func (p *PushGateway) fetchExisting(ctx context.Context) map[string]*dto.MetricFamily {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.metricsURL, http.NoBody)
	if err != nil {
		p.logger.Logf("could not create GET request for existing metrics: %v", err)
		return nil
	}

	resp, err := p.client.Do(req)
	if err != nil {
		p.logger.Logf("could not fetch existing metrics (first run?): %v", err)
		return nil
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		p.logger.Logf("Pushgateway returned %d on GET /metrics, treating as empty", resp.StatusCode)
		return nil
	}

	parser := expfmt.NewTextParser(model.LegacyValidation)

	families, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		p.logger.Logf("could not parse existing metrics: %v", err)
		return nil
	}

	return families
}

// mergeMetrics combines existing (from Pushgateway) and local (from this run)
// metric families. Counters and histograms are summed; gauges use the local
// value (latest wins).
func mergeMetrics(existing map[string]*dto.MetricFamily, local []*dto.MetricFamily) []*dto.MetricFamily {
	result := make([]*dto.MetricFamily, 0, len(local))

	for _, lf := range local {
		name := lf.GetName()
		ef, found := existing[name]

		if !found {
			result = append(result, lf)
			continue
		}

		merged := mergeFamilies(ef, lf)
		result = append(result, merged)
	}

	return result
}

// mergeFamilies merges two metric families with the same name.
func mergeFamilies(existing, local *dto.MetricFamily) *dto.MetricFamily {
	mtype := local.GetType()

	switch mtype {
	case dto.MetricType_COUNTER:
		return mergeCounterFamily(existing, local)

	case dto.MetricType_HISTOGRAM:
		return mergeHistogramFamily(existing, local)

	case dto.MetricType_GAUGE, dto.MetricType_SUMMARY,
		dto.MetricType_UNTYPED, dto.MetricType_GAUGE_HISTOGRAM:
		return local
	}

	return local
}

// mergeCounterFamily sums counter values for metrics with matching labels.
func mergeCounterFamily(existing, local *dto.MetricFamily) *dto.MetricFamily {
	existingByLabels := indexByLabels(existing.GetMetric())

	for _, lm := range local.GetMetric() {
		key := labelKey(lm.GetLabel())
		if em, ok := existingByLabels[key]; ok && em.GetCounter() != nil && lm.GetCounter() != nil {
			sum := em.GetCounter().GetValue() + lm.GetCounter().GetValue()
			lm.Counter.Value = &sum
		}
	}

	return local
}

// mergeHistogramFamily sums histogram bucket counts, _sum, and _count for
// metrics with matching labels.
func mergeHistogramFamily(existing, local *dto.MetricFamily) *dto.MetricFamily {
	existingByLabels := indexByLabels(existing.GetMetric())

	for _, lm := range local.GetMetric() {
		key := labelKey(lm.GetLabel())

		em, ok := existingByLabels[key]
		if !ok || em.GetHistogram() == nil || lm.GetHistogram() == nil {
			continue
		}

		eh := em.GetHistogram()
		lh := lm.GetHistogram()

		// Sum sample count and sample sum
		count := eh.GetSampleCount() + lh.GetSampleCount()
		lh.SampleCount = &count

		sum := eh.GetSampleSum() + lh.GetSampleSum()
		lh.SampleSum = &sum

		// Sum bucket cumulative counts
		existingBuckets := make(map[float64]uint64)
		for _, b := range eh.GetBucket() {
			existingBuckets[b.GetUpperBound()] = b.GetCumulativeCount()
		}

		for _, b := range lh.GetBucket() {
			if ec, ok := existingBuckets[b.GetUpperBound()]; ok {
				merged := ec + b.GetCumulativeCount()
				b.CumulativeCount = &merged
			}
		}
	}

	return local
}

// indexByLabels creates a lookup map from label signature to metric.
func indexByLabels(metrics []*dto.Metric) map[string]*dto.Metric {
	m := make(map[string]*dto.Metric, len(metrics))
	for _, metric := range metrics {
		m[labelKey(metric.GetLabel())] = metric
	}

	return m
}

// labelKey returns a canonical string key for a set of label pairs,
// used to match metrics across existing and local families.
// It excludes labels injected by the Pushgateway (job, instance) and
// OTel scope labels so that existing and local metrics can be matched
// by their application-defined labels only.
func labelKey(labels []*dto.LabelPair) string {
	var b strings.Builder

	first := true

	for _, lp := range labels {
		name := lp.GetName()
		if isExternalLabel(name) {
			continue
		}

		if !first {
			b.WriteByte(',')
		}

		first = false

		b.WriteString(name)
		b.WriteByte('=')
		b.WriteString(lp.GetValue())
	}

	return b.String()
}

// isExternalLabel returns true for labels injected by the Pushgateway
// or OTel SDK that are not part of the application's own metric labels.
func isExternalLabel(name string) bool {
	switch name {
	case "job", "instance":
		return true
	}

	return strings.HasPrefix(name, "otel_scope_")
}

// put encodes the merged metric families as Prometheus text format and PUTs
// them to the Pushgateway.
func (p *PushGateway) put(ctx context.Context, families []*dto.MetricFamily) error {
	var buf bytes.Buffer

	for _, mf := range families {
		if _, err := expfmt.MetricFamilyToText(&buf, mf); err != nil {
			return fmt.Errorf("encoding metric family %q: %w", mf.GetName(), err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, p.pushURL, &buf)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", string(expfmt.NewFormat(expfmt.TypeTextPlain)))

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// Drain body so the connection can be reused
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("%w: %d", errUnexpectedStatus, resp.StatusCode)
	}

	return nil
}
