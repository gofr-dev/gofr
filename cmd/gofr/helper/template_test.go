package helper

import (
	"strings"
	"testing"
)

func TestHelpString(t *testing.T) {
	type args struct {
		value Help
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{"only usage specified", args{Help{Usage: "test"}}, "test"},
		{"only flag specified", args{Help{Flag: "test"}}, "test"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := Generate(tt.args.value); !strings.Contains(got, tt.want) {
				t.Errorf("Generate() = %v, want %v", got, tt.want)
			}
		})
	}
}
