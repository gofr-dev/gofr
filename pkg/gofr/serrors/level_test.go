package serrors

import "testing"

func TestLevel_GetErrorLevel(t *testing.T) {
	tests := []struct {
		name  string
		level Level
		want  string
	}{
		{"info level", INFO, "INFO"},
		{"warn level", WARNING, "WARNING"},
		{"error level", ERROR, "ERROR"},
		{"critical level", CRITICAL, "CRITICAL"},

		{"positive invalid level", Level(100), "UNKNOWN"},

		{"negative invalid level", Level(-1), "UNKNOWN"},
		{"out of range invalid level", Level(9999999999999), "UNKNOWN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.GetErrorLevel(); got != tt.want {
				t.Errorf("GetErrorLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}
