package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSASLMechanism_Success(t *testing.T) {
	tests := []struct {
		name         string
		mechanism    string
		username     string
		password     string
		expectedName string
	}{
		{
			name:         "PLAIN uppercase",
			mechanism:    "PLAIN",
			username:     "user",
			password:     "pass",
			expectedName: "PLAIN",
		},
		{
			name:         "plain lowercase",
			mechanism:    "plain",
			username:     "user",
			password:     "pass",
			expectedName: "PLAIN",
		},
		{
			name:         "SCRAM-SHA-256",
			mechanism:    "SCRAM-SHA-256",
			username:     "user",
			password:     "pass",
			expectedName: "SCRAM-SHA-256",
		},
		{
			name:         "SCRAM-SHA-512",
			mechanism:    "SCRAM-SHA-512",
			username:     "user",
			password:     "pass",
			expectedName: "SCRAM-SHA-512",
		},
	}

	for _, tc := range tests {
		mechanism, err := getSASLMechanism(tc.mechanism, tc.username, tc.password)

		require.NoError(t, err, "unexpected error: %v", err)
		assert.Equal(t, tc.expectedName, mechanism.Name(),
			"expected mechanism name %q, got %q", tc.expectedName, mechanism.Name())
	}
}

func TestGetSASLMechanism_Failure(t *testing.T) {
	_, err := getSASLMechanism("FOO", "user", "pass")

	require.Error(t, err, "expected an error for unsupported mechanism but got none")
	assert.Contains(t, err.Error(), "unsupported SASL mechanism",
		"unexpected error message: %v", err)
}
