package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level          level
		expectedString string
	}{
		{DEBUG, levelDEBUG},
		{INFO, levelINFO},
		{NOTICE, levelNOTICE},
		{WARN, levelWARN},
		{ERROR, levelERROR},
		{FATAL, levelFATAL},
		{level(99), ""}, // Test default case
	}

	for i, tc := range tests {
		assert.Equal(t, tc.expectedString, tc.level.String(), "TEST[%d], Failed.\n", i)
	}
}

func TestLevelColor(t *testing.T) {
	tests := []struct {
		level         level
		expectedColor uint
	}{
		{ERROR, 31},
		{FATAL, 31},
		{WARN, 33},
		{NOTICE, 33},
		{INFO, 36},
		{DEBUG, 36},
		{level(99), 37}, // Test default case
	}

	for i, tc := range tests {
		assert.Equal(t, tc.expectedColor, tc.level.color(), "TEST[%d], Failed.\n", i)
	}
}
