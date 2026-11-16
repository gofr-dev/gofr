//go:build !gofr_nopubsub

package container

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource/pubsub/google"
	"gofr.dev/pkg/gofr/datasource/pubsub/kafka"
	"gofr.dev/pkg/gofr/logging"
)

func TestContainer_googlePubSubInt(t *testing.T) {
	c := &Container{Logger: logging.NewMockLogger(logging.ERROR)}

	testCases := []struct {
		desc       string
		configs    map[string]string
		key        string
		defaultVal int
		expected   int
	}{
		{
			desc:       "valid value is parsed",
			configs:    map[string]string{"GOOGLE_MAX_OUTSTANDING_MESSAGES": "100"},
			key:        "GOOGLE_MAX_OUTSTANDING_MESSAGES",
			defaultVal: google.DefaultMaxOutstandingMessages,
			expected:   100,
		},
		{
			desc:       "unset falls back to the GoFr default",
			configs:    map[string]string{},
			key:        "GOOGLE_MAX_OUTSTANDING_MESSAGES",
			defaultVal: google.DefaultMaxOutstandingMessages,
			expected:   google.DefaultMaxOutstandingMessages,
		},
		{
			desc:       "invalid value falls back to the GoFr default",
			configs:    map[string]string{"GOOGLE_NUM_GOROUTINES": "not-a-number"},
			key:        "GOOGLE_NUM_GOROUTINES",
			defaultVal: google.DefaultNumGoroutines,
			expected:   google.DefaultNumGoroutines,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := c.googlePubSubInt(config.NewMockConfig(tc.configs), tc.key, tc.defaultVal)

			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestContainer_googleConfigFromEnv(t *testing.T) {
	c := &Container{Logger: logging.NewMockLogger(logging.ERROR)}

	cfg := c.googleConfigFromEnv(config.NewMockConfig(map[string]string{
		"GOOGLE_PROJECT_ID":               "proj",
		"GOOGLE_SUBSCRIPTION_NAME":        "sub",
		"GOOGLE_MAX_OUTSTANDING_MESSAGES": "11",
		"GOOGLE_MAX_OUTSTANDING_BYTES":    "22",
		"GOOGLE_NUM_GOROUTINES":           "33",
	}))

	// Distinct values so a field fed from the wrong env key (e.g. NumGoroutines) fails.
	assert.Equal(t, "proj", cfg.ProjectID)
	assert.Equal(t, "sub", cfg.SubscriptionName)
	assert.Equal(t, 11, cfg.MaxOutstandingMessages)
	assert.Equal(t, 22, cfg.MaxOutstandingBytes)
	assert.Equal(t, 33, cfg.NumGoroutines)
}

func TestContainer_googleConfigFromEnv_Defaults(t *testing.T) {
	c := &Container{Logger: logging.NewMockLogger(logging.ERROR)}

	cfg := c.googleConfigFromEnv(config.NewMockConfig(nil))

	assert.Equal(t, google.DefaultMaxOutstandingMessages, cfg.MaxOutstandingMessages)
	assert.Equal(t, google.DefaultMaxOutstandingBytes, cfg.MaxOutstandingBytes)
	assert.Equal(t, google.DefaultNumGoroutines, cfg.NumGoroutines)
}

func TestContainer_kafkaConfigFromEnv(t *testing.T) {
	tests := []struct {
		desc         string
		configs      map[string]string
		expOffSet    int
		expBatchSize int
		expBytes     int
		expTimeout   int
	}{
		{
			desc:         "unset values use the defaults",
			configs:      map[string]string{},
			expOffSet:    -1,
			expBatchSize: kafka.DefaultBatchSize,
			expBytes:     kafka.DefaultBatchBytes,
			expTimeout:   kafka.DefaultBatchTimeout,
		},
		{
			desc: "valid values pass through",
			configs: map[string]string{
				"PUBSUB_OFFSET":       "0",
				"KAFKA_BATCH_SIZE":    "5",
				"KAFKA_BATCH_BYTES":   "2048",
				"KAFKA_BATCH_TIMEOUT": "250",
			},
			expOffSet:    0,
			expBatchSize: 5,
			expBytes:     2048,
			expTimeout:   250,
		},
		{
			desc: "invalid values fall back to the documented defaults",
			configs: map[string]string{
				"PUBSUB_OFFSET":       "latest",
				"KAFKA_BATCH_SIZE":    "many",
				"KAFKA_BATCH_BYTES":   "1MB",
				"KAFKA_BATCH_TIMEOUT": "1s",
			},
			expOffSet:    -1,
			expBatchSize: kafka.DefaultBatchSize,
			expBytes:     kafka.DefaultBatchBytes,
			expTimeout:   kafka.DefaultBatchTimeout,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			c := &Container{Logger: logging.NewMockLogger(logging.ERROR)}
			tc.configs["PUBSUB_BROKER"] = "localhost:9092"

			got := c.kafkaConfigFromEnv(config.NewMockConfig(tc.configs))

			assert.Equal(t, tc.expOffSet, got.OffSet)
			assert.Equal(t, tc.expBatchSize, got.BatchSize)
			assert.Equal(t, tc.expBytes, got.BatchBytes)
			assert.Equal(t, tc.expTimeout, got.BatchTimeout)
		})
	}
}
