package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level          Level
		expectedString string
	}{
		{DEBUG, levelDEBUG},
		{INFO, levelINFO},
		{NOTICE, levelNOTICE},
		{WARN, levelWARN},
		{ERROR, levelERROR},
		{FATAL, levelFATAL},
		{Level(99), ""}, // Test default case
	}

	for i, tc := range tests {
		assert.Equal(t, tc.expectedString, tc.level.String(), "TEST[%d], Failed.\n", i)
	}
}

func TestLevelColor(t *testing.T) {
	tests := []struct {
		level         Level
		expectedColor uint
	}{
		{ERROR, 160},
		{FATAL, 160},
		{WARN, 220},
		{NOTICE, 220},
		{INFO, 6},
		{DEBUG, 8},
		{Level(99), 37}, // Test default case
	}

	for i, tc := range tests {
		assert.Equal(t, tc.expectedColor, tc.level.color(), "TEST[%d], Failed.", i)
	}
}

func TestGetLevelFromString(t *testing.T) {
	tests := []struct {
		desc     string
		input    string
		expected Level
	}{
		{
			desc:     "DebugLevel",
			input:    "DEBUG",
			expected: DEBUG,
		},
		{
			desc:     "InfoLevel",
			input:    "INFO",
			expected: INFO,
		},
		{
			desc:     "NoticeLevel",
			input:    "NOTICE",
			expected: NOTICE,
		},
		{
			desc:     "WarnLevel",
			input:    "WARN",
			expected: WARN,
		},
		{
			desc:     "ErrorLevel",
			input:    "ERROR",
			expected: ERROR,
		},
		{
			desc:     "FatalLevel",
			input:    "FATAL",
			expected: FATAL,
		},
		{
			desc:     "DefaultLevel",
			input:    "UNKNOWN",
			expected: INFO,
		},
	}

	for i, tc := range tests {
		actual := GetLevelFromString(tc.input)

		assert.Equal(t, tc.expected, actual, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
