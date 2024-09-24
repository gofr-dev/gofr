package kafka

import (
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/health"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestKafkaClient_HealthStatusUP(t *testing.T) {
	ctrl := gomock.NewController(t)

	conn := NewMockConnection(ctrl)
	reader := NewMockReader(ctrl)

	writer := NewMockWriter(ctrl)

	client := &kafkaClient{
		conn:   conn,
		reader: map[string]Reader{"test": reader},
		writer: writer,
	}

	expectedHealth := health.Health{
		Status: health.StatusUp,
		Details: map[string]interface{}{
			"host":    "",
			"backend": "KAFKA",
		},
	}

	conn.EXPECT().Controller().Return(kafka.Broker{}, nil)
	writer.EXPECT().Stats().Return(kafka.WriterStats{Topic: "test"})
	reader.EXPECT().Stats().Return(kafka.ReaderStats{Topic: "test"})

	h := client.Health()

	assert.Equal(t, expectedHealth.Details["host"], h.Details["host"])
	assert.Equal(t, expectedHealth.Details["backend"], h.Details["backend"])
	assert.Equal(t, expectedHealth.Status, h.Status)
}

func TestKafkaClient_HealthStatusDown(t *testing.T) {
	ctrl := gomock.NewController(t)

	conn := NewMockConnection(ctrl)
	reader := NewMockReader(ctrl)
	writer := NewMockWriter(ctrl)

	client := &kafkaClient{
		conn:   conn,
		reader: map[string]Reader{"test": reader},
		writer: writer,
	}

	conn.EXPECT().Controller().Return(kafka.Broker{}, testutil.CustomError{ErrorMessage: "connection failed"})
	writer.EXPECT().Stats().Return(kafka.WriterStats{Topic: "test"})
	reader.EXPECT().Stats().Return(kafka.ReaderStats{Topic: "test"})

	h := client.Health()

	assert.Equal(t, health.StatusDown, h.Status, "Status should be DOWN")
	assert.Equal(t, "KAFKA", h.Details["backend"], "Backend should be KAFKA")
	assert.Equal(t, "", h.Details["host"], "Host should be empty")

	// Check if readers and writers exist in the health details
	assert.Contains(t, h.Details, "readers", "Health should contain readers")
	assert.Contains(t, h.Details, "writers", "Health should contain writers")

	// Check the structure of readers
	readers, ok := h.Details["readers"].([]interface{})
	assert.True(t, ok, "Readers should be a slice")
	assert.Len(t, readers, 1, "There should be one reader")
	readerStats, ok := readers[0].(map[string]interface{})
	assert.True(t, ok, "Reader stats should be a map")
	assert.Equal(t, "test", readerStats["Topic"], "Reader topic should be 'test'")

	// Check the structure of writers
	writers, ok := h.Details["writers"].(map[string]interface{})
	assert.True(t, ok, "Writers should be a map")
	assert.Equal(t, "test", writers["Topic"], "Writer topic should be 'test'")

	// Log the entire health object for debugging
	t.Logf("Actual health: %+v", h)
}

func TestKafkaClient_getWriterStatsAsMap(t *testing.T) {
	ctrl := gomock.NewController(t)

	writer := NewMockWriter(ctrl)

	client := &kafkaClient{
		logger: logging.NewMockLogger(logging.DEBUG),
		writer: writer,
	}

	writer.EXPECT().Stats().Return(kafka.WriterStats{Topic: "test"})

	writerStats := client.getWriterStatsAsMap()

	assert.NotNil(t, writerStats)
}

func TestKafkaClient_getReaderStatsAsMap(t *testing.T) {
	ctrl := gomock.NewController(t)

	reader := NewMockReader(ctrl)

	client := &kafkaClient{
		logger: logging.NewMockLogger(logging.DEBUG),
		reader: map[string]Reader{"test": reader},
	}

	reader.EXPECT().Stats().Return(kafka.ReaderStats{Topic: "test"})

	writerStats := client.getReaderStatsAsMap()

	assert.NotNil(t, writerStats)
}

func TestKafkaClient_convertStructToMap(t *testing.T) {
	testCases := []struct {
		desc   string
		input  interface{}
		output interface{}
	}{
		{"unmarshal error", make(chan int), nil},
	}

	for _, v := range testCases {
		err := convertStructToMap(v.input, v.output)

		require.ErrorContains(t, err, "json: unsupported type: chan int")
	}
}
