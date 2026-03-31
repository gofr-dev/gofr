package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAPIKeyConfig(t *testing.T) {
	testCases := []struct {
		name    string
		apiKey  string
		wantErr bool
		errMsg  string
	}{
		{name: "empty key", apiKey: "", wantErr: true, errMsg: "api key is required"},
		{name: "whitespace key", apiKey: "  ", wantErr: true, errMsg: "api key is required"},
		{name: "valid key", apiKey: "my-api-key"},
		{name: "trimmed whitespace", apiKey: "  my-api-key  "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opt, err := NewAPIKeyConfig(tc.apiKey)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
				assert.Nil(t, opt)

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, opt)
		})
	}
}

func TestAPIKeyConfig_GetHeaderKey(t *testing.T) {
	cfg := &apiKeyConfig{apiKey: "key"}
	assert.Equal(t, "X-Api-Key", cfg.GetHeaderKey())
}

func TestAPIKeyConfig_GetHeaderValue(t *testing.T) {
	cfg := &apiKeyConfig{apiKey: "my-secret-key"}

	value, err := cfg.GetHeaderValue(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "my-secret-key", value)
}
