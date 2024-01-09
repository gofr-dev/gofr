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
		{ERROR, 31},
		{FATAL, 31},
		{WARN, 33},
		{NOTICE, 33},
		{INFO, 36},
		{DEBUG, 36},
		{Level(99), 37}, // Test default case
	}

	for i, tc := range tests {
		assert.Equal(t, tc.expectedColor, tc.level.color(), "TEST[%d], Failed.\n", i)
	}
}
