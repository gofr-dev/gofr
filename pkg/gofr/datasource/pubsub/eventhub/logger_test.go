package eventhub

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_PrettyPrint(t *testing.T) {
	queryLog := Log{
		Mode:          "PUB",
		MessageValue:  `{"myorder":"1"}`,
		Topic:         "test-topic",
		Host:          "localhost",
		PubSubBackend: "AZHUB",
		Time:          10,
	}

	logger := NewMockLogger(gomock.NewController(t))

	logger.EXPECT().Log(gomock.Any())

	logger.Log(queryLog)

	b := make([]byte, 100)

	writer := bytes.NewBuffer(b)

	queryLog.PrettyPrint(writer)

	require.Contains(t, writer.String(), "test-topic")
	require.Contains(t, writer.String(), "AZHUB")
	require.Contains(t, writer.String(), `{"myorder":"1"}`)

	require.True(t, logger.ctrl.Satisfied(), "Test_PrettyPrint Failed!")
}
