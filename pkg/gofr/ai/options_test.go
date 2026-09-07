package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyOptions(t *testing.T) {
	spec := []ToolSpec{{Name: "a"}}

	got := ApplyOptions(
		WithTools(spec),
		WithTemperature(0.5),
		WithMaxTokens(256),
		nil, // a nil option is skipped, not a panic
	)

	assert.Equal(t, spec, got.Tools)
	require.NotNil(t, got.Temperature)
	assert.InEpsilon(t, 0.5, *got.Temperature, 1e-9)
	require.NotNil(t, got.MaxTokens)
	assert.Equal(t, 256, *got.MaxTokens)
}

func TestApplyOptions_Empty(t *testing.T) {
	got := ApplyOptions()

	assert.Nil(t, got.Tools)
	assert.Nil(t, got.Temperature)
	assert.Nil(t, got.MaxTokens)
}

func TestAccess_String(t *testing.T) {
	assert.Equal(t, "read", ReadOnly.String())
	assert.Equal(t, "write", Write.String())
}

func TestResult_JSON(t *testing.T) {
	r := NewResult(map[string]int{"n": 1})
	assert.Equal(t, map[string]int{"n": 1}, r.Value())

	b, err := r.JSON()
	require.NoError(t, err)
	assert.JSONEq(t, `{"n":1}`, string(b))
}
