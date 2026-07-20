package gofr

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/testutil"
)

// TestApp_AddLLM_Named registers a default model and two named models and asserts each resolves
// independently, an unknown name resolves to nil (not a panic), and every model is reported on the
// health endpoint. This exercises WithName and the ctx.LLM(name) selector.
func TestApp_AddLLM_Named(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	def := &testLLM{}
	fast := &testLLM{}
	strong := &testLLM{}

	app.AddLLM(def)
	app.AddLLM(fast, WithName("fast"))
	app.AddLLM(strong, WithName("strong"))

	assert.Same(t, def, app.container.LLMModel())
	assert.Same(t, fast, app.container.LLMModel("fast"))
	assert.Same(t, strong, app.container.LLMModel("strong"))

	assert.NotNil(t, app.container.LLM())
	assert.NotNil(t, app.container.LLM("fast"))
	assert.Nil(t, app.container.LLM("nonexistent"))

	details, ok := app.container.Health(context.Background()).(map[string]any)
	assert.True(t, ok)
	assert.Contains(t, details, "llm")
	assert.Contains(t, details, "llm_fast")
	assert.Contains(t, details, "llm_strong")
}
