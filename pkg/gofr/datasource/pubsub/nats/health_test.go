package nats_test

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/logging"
)

func TestNATSClient_HealthStatusUP(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockConn := NewMockConnection(ctrl)
	mockJS := NewMockJetStreamContext(ctrl)

	client := &natsClient{
		conn: mockConn,
		js:   mockJS,
		config: Config{
			Server: "nats://localhost:4222",
		},
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	mockConn.EXPECT().Status().Return(nats.CONNECTED)
	mockConn.EXPECT().Opts().Return(&nats.Options{})
	mockJS.EXPECT().AccountInfo(gomock.Any()).Return(&jetstream.AccountInfo{}, nil)

	health := client.Health()

	assert.Equal(t, datasource.StatusUp, health.Status)
	assert.Equal(t, "nats://localhost:4222", health.Details["host"])
	assert.Equal(t, "NATS", health.Details["backend"])
	assert.Equal(t, "CONNECTED", health.Details["connection_status"])
	assert.Equal(t, true, health.Details["jetstream_enabled"])
	assert.NotNil(t, health.Details["jetstream_info"])
}

func TestNATSClient_HealthStatusDown(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockConn := NewMockConnection(ctrl)

	client := &natsClient{
		conn: mockConn,
		config: Config{
			Server: "nats://localhost:4222",
		},
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	mockConn.EXPECT().Status().Return(nats.CLOSED)

	health := client.Health()

	assert.Equal(t, datasource.StatusDown, health.Status)
	assert.Equal(t, "nats://localhost:4222", health.Details["host"])
	assert.Equal(t, "NATS", health.Details["backend"])
	assert.Equal(t, "CLOSED", health.Details["connection_status"])
	assert.Equal(t, false, health.Details["jetstream_enabled"])
}

func TestNATSClient_getJetStreamInfo(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockJS := NewMockJetStreamContext(ctrl)

	client := &natsClient{
		js:     mockJS,
		conn:   &nats.Conn{Opts: &nats.Options{}},
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	mockJS.EXPECT().AccountInfo(gomock.Any()).Return(&jetstream.AccountInfo{
		Memory:    1024,
		Storage:   2048,
		Streams:   5,
		Consumers: 10,
	}, nil)

	info, err := client.getJetStreamInfo()

	require.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, float64(1024), info["memory"])
	assert.Equal(t, float64(2048), info["storage"])
	assert.Equal(t, float64(5), info["streams"])
	assert.Equal(t, float64(10), info["consumers"])
}

func TestConvertStructToMap(t *testing.T) {
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
