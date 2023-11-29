package pubsub

import (
	"github.com/prometheus/client_golang/prometheus"

	"gofr.dev/pkg"
)

//nolint:gochecknoglobals // The declared global variable can be accessed across multiple functions
var (
	subscribeReceiveCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: pkg.FrameworkMetricsPrefix + "pubsub_receive_count",
		Help: "Total number of subscribe operation",
	}, []string{"topic", "consumerGroup"})

	subscribeSuccessCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: pkg.FrameworkMetricsPrefix + "pubsub_success_count",
		Help: "Total number of successful subscribe operation",
	}, []string{"topic", "consumerGroup"})

	subscribeFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: pkg.FrameworkMetricsPrefix + "pubsub_failure_count",
		Help: "Total number of failed subscribe operation",
	}, []string{"topic", "consumerGroup"})

	publishSuccessCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: pkg.FrameworkMetricsPrefix + "pubsub_publish_success_count",
		Help: "Counter for the number of messages successfully published",
	}, []string{"topic", "consumerGroup"})

	publishFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: pkg.FrameworkMetricsPrefix + "pubsub_publish_failure_count",
		Help: "Counter for the number of failed publish operations",
	}, []string{"topic", "consumerGroup"})

	publishTotalCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: pkg.FrameworkMetricsPrefix + "pubsub_publish_total_count",
		Help: "Counter for the total number of publish operations",
	}, []string{"topic", "consumerGroup"})
)

func RegisterMetrics() {
	_ = prometheus.Register(subscribeReceiveCount)
	_ = prometheus.Register(subscribeFailureCount)
	_ = prometheus.Register(subscribeSuccessCount)
	_ = prometheus.Register(publishFailureCount)
	_ = prometheus.Register(publishSuccessCount)
	_ = prometheus.Register(publishTotalCount)
}

func PublishTotalCount(label ...string) {
	publishTotalCount.WithLabelValues(label...).Inc()
}

func PublishSuccessCount(label ...string) {
	publishSuccessCount.WithLabelValues(label...).Inc()
}

func PublishFailureCount(label ...string) {
	publishFailureCount.WithLabelValues(label...).Inc()
}

func SubscribeReceiveCount(label ...string) {
	subscribeReceiveCount.WithLabelValues(label...).Inc()
}

func SubscribeFailureCount(label ...string) {
	subscribeFailureCount.WithLabelValues(label...).Inc()
}

func SubscribeSuccessCount(label ...string) {
	subscribeSuccessCount.WithLabelValues(label...).Inc()
}
