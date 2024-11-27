package dgraph

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_PrettyPrint(t *testing.T) {
	queryLog := QueryLog{
		Type:     "GET",
		Duration: 12345,
	}

	logger := NewMockLogger(gomock.NewController(t))

	logger.EXPECT().Debug(gomock.Any())

	queryLog.PrettyPrint(logger)

	require.True(t, logger.ctrl.Satisfied(), "Test_PrettyPrint Failed!")
}
