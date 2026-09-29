package nats

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr/logging"
)

func TestNew_NilConfig(t *testing.T) {
	w := New(nil, logging.NewMockLogger(logging.DEBUG))

	require.NotNil(t, w.Client.Config, "a nil config must be replaced with an empty one")
	assert.Equal(t, &Config{}, w.Client.Config)
	assert.NotNil(t, w.Client.subManager)
}

func TestValidateConfigs(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *Config
		expErr error
	}{
		{
			name:   "missing server",
			cfg:    &Config{Stream: StreamConfig{Subjects: []string{"s"}}, Consumer: "c"},
			expErr: errServerNotProvided,
		},
		{
			name:   "missing subjects",
			cfg:    &Config{Server: NATSServer, Consumer: "c"},
			expErr: errSubjectsNotProvided,
		},
		{
			name:   "missing consumer",
			cfg:    &Config{Server: NATSServer, Stream: StreamConfig{Subjects: []string{"s"}}},
			expErr: errConsumerNotProvided,
		},
		{
			name:   "valid",
			cfg:    &Config{Server: NATSServer, Stream: StreamConfig{Subjects: []string{"s"}}, Consumer: "c"},
			expErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expErr, validateConfigs(tt.cfg))
		})
	}
}
