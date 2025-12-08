package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogPanic(t *testing.T) {
	testCases := []struct {
		desc          string
		panicVal      any
		expectedError string
	}{
		{"panic with string", "something went wrong", "something went wrong"},
		{"panic with error", errors.New("runtime error"), "runtime error"},
		{"panic with unknown type", 123, "Unknown panic type"},
		{"nil panic", nil, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var buf bytes.Buffer
			logger := &logger{
				level:      ERROR,
				normalOut:  &buf,
				errorOut:   &buf,
				isTerminal: false,
			}

			LogPanic(tc.panicVal, logger)

			if tc.panicVal == nil {
				assert.Empty(t, buf.String())
				return
			}

			var entry map[string]interface{}
			err := json.Unmarshal(buf.Bytes(), &entry)
			assert.NoError(t, err)

			panicLogMap, ok := entry["message"].(map[string]interface{})
			assert.True(t, ok)
			assert.Equal(t, tc.expectedError, panicLogMap["error"])
			assert.NotEmpty(t, panicLogMap["stack_trace"])
		})
	}
}
