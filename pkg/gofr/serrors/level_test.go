package serrors

import "testing"

const (
	TestUnknown  = "UNKNOWN"
	TestError    = "ERROR"
	TestWarning  = "WARNING"
	TestInfo     = "INFO"
	TestCritical = "CRITICAL"
)

func TestLevel_GetErrorLevel(t *testing.T) {
	tests := []struct {
		name  string
		level Level
		want  string
	}{
		{"info level", INFO, TestInfo},
		{"warn level", WARNING, TestWarning},
		{"error level", ERROR, TestError},
		{"critical level", CRITICAL, TestCritical},

		{"positive invalid level", Level(100), TestUnknown},

		{"negative invalid level", Level(-1), TestUnknown},
		{"out of range invalid level", Level(9999999999999), TestUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.GetErrorLevel(); got != tt.want {
				t.Errorf("GetErrorLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}
