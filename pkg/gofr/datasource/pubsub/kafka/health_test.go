package kafka

import (
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource"
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

	expectedHealth := datasource.Health{
		Status: datasource.StatusUp,
		Details: map[string]interface{}{
			"host":    "",
			"backend": "KAFKA",
		},
	}

	conn.EXPECT().Controller().Return(kafka.Broker{}, nil)
	writer.EXPECT().Stats().Return(kafka.WriterStats{Topic: "test"})
	reader.EXPECT().Stats().Return(kafka.ReaderStats{Topic: "test"})

	health := client.Health()

	assert.Equal(t, expectedHealth.Details["host"], health.Details["host"])
	assert.Equal(t, expectedHealth.Details["backend"], health.Details["backend"])
	assert.Equal(t, expectedHealth.Status, health.Status)
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

	expectedHealth := datasource.Health{
		Status: datasource.StatusDown,
		Details: map[string]interface{}{
			"host":    "",
			"backend": "KAFKA",
		},
	}

	conn.EXPECT().Controller().Return(kafka.Broker{}, testutil.CustomError{ErrorMessage: "connection failed"})
	writer.EXPECT().Stats().Return(kafka.WriterStats{Topic: "test"})
	reader.EXPECT().Stats().Return(kafka.ReaderStats{Topic: "test"})

	health := client.Health()

	assert.Equal(t, expectedHealth.Details["host"], health.Details["host"])
	assert.Equal(t, expectedHealth.Details["backend"], health.Details["backend"])
	assert.Equal(t, expectedHealth.Status, health.Status)
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

func TestKafkaClint_convertStructToMap(t *testing.T) {
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
