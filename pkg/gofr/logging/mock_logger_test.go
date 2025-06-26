package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/testutil"
)

func Test_NewMockLogger(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		logger := NewMockLogger(DEBUG)

		logger.Info("INFO Log")
		logger.Infof("Info Log with Format Value: %v", "infof")

		logger.Warn("WARN Log")
		logger.Warnf("Warn Log with Format Value: %v", "warnf")

		logger.Notice("NOTICE Log")
		logger.Noticef("Notice Log with Format Value: %v", "noticef")

		logger.Log("Direct Log")
		logger.Logf("Direct Log with Format Value: %v", "logf")

		logger.Debug("DEBUG Log")
		logger.Debugf("Debug Log with Format Value: %v", "debugf")
	})

	assert.Contains(t, logs, "INFO Log")
	assert.Contains(t, logs, "Info Log with Format Value: infof")

	assert.Contains(t, logs, "WARN Log")
	assert.Contains(t, logs, "Warn Log with Format Value: warnf")

	assert.Contains(t, logs, "NOTICE Log")
	assert.Contains(t, logs, "Notice Log with Format Value: noticef")

	assert.Contains(t, logs, "Direct Log")
	assert.Contains(t, logs, "Direct Log with Format Value: logf")

	assert.Contains(t, logs, "DEBUG Log")
	assert.Contains(t, logs, "Debug Log with Format Value: debugf")
}

func Test_NewMockLoggerErrorLogs(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		logger := NewMockLogger(DEBUG)

		logger.Error("ERROR Log")
		logger.Errorf("error Log with Format Value: %v", "errorf")
	})

	assert.Contains(t, logs, "ERROR Log")
	assert.Contains(t, logs, "error Log with Format Value: errorf")
}
