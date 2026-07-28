package gcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/metrics/exporters"
)

type testLogger struct{ warned bool }

func (*testLogger) Debug(...any)           {}
func (*testLogger) Infof(string, ...any)   {}
func (l *testLogger) Warnf(string, ...any) { l.warned = true }
func (*testLogger) Errorf(string, ...any)  {}

// writeADC points GOOGLE_APPLICATION_CREDENTIALS at a well-formed authorized_user
// credentials file so FindDefaultCredentials resolves without any network call.
func writeADC(t *testing.T) {
	t.Helper()

	adc := map[string]string{
		"type":          "authorized_user",
		"client_id":     "test-id",
		"client_secret": "test-secret",
		"refresh_token": "test-token",
	}

	b, err := json.Marshal(adc)
	if err != nil {
		t.Fatal(err)
	}

	p := filepath.Join(t.TempDir(), "adc.json")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", p)
}

func Test_buildReader(t *testing.T) {
	writeADC(t)

	tests := []struct {
		name     string
		cfg      exporters.Config
		wantWarn bool
	}{
		{"cumulative default endpoint", exporters.Config{Interval: time.Second}, false},
		{"explicit endpoint", exporters.Config{Endpoint: defaultEndpoint, Interval: time.Second}, false},
		{"delta preference warns and is ignored", exporters.Config{Interval: time.Second, Temporality: "delta"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := &testLogger{}

			r, err := buildReader(context.Background(), &tc.cfg, l)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if r == nil {
				t.Fatal("expected non-nil reader")
			}

			if l.warned != tc.wantWarn {
				t.Errorf("warn = %v, want %v", l.warned, tc.wantWarn)
			}

			_ = r.Shutdown(context.Background())
		})
	}
}
