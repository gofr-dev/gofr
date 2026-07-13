package gofr

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/ai/llm"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/testutil"
)

type testLLM struct {
	configUsed bool
	connected  bool
	status     string
}

func (*testLLM) Chat(context.Context, []ai.Message, ...ai.Option) (*ai.Response, error) {
	return &ai.Response{}, nil
}

func (t *testLLM) HealthCheck(context.Context) datasource.Health {
	if t.status == "" {
		t.status = datasource.StatusUp
	}

	return datasource.Health{Status: t.status}
}

func (*testLLM) Name() string { return "test-llm" }

func (t *testLLM) UseConfig(any) { t.configUsed = true }
func (t *testLLM) Connect()      { t.connected = true }

func TestApp_AddLLM(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()
	m := &testLLM{}

	app.AddLLM(m)

	assert.True(t, m.configUsed, "AddLLM must deliver config via UseConfig")
	assert.True(t, m.connected, "AddLLM must call Connect")
	require.NotNil(t, app.container.LLM(), "ctx.LLM() must resolve the added model")
}

func TestApp_AddLLM_Health(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()
	app.AddLLM(&testLLM{status: datasource.StatusDown})

	health, ok := app.container.Health(t.Context()).(map[string]any)
	require.True(t, ok)

	llmHealth, ok := health["llm"].(datasource.Health)
	require.True(t, ok, "an added model reports on the health endpoint")
	assert.Equal(t, datasource.StatusDown, llmHealth.Status)
	assert.Equal(t, "DEGRADED", health["status"], "a down model degrades the aggregate health")
}

func TestApp_AddLLM_SecondModelReplacesWithoutError(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()
	first, second := &testLLM{}, &testLLM{}

	app.AddLLM(first)
	app.AddLLM(second) // metrics registered once; second model replaces the first

	assert.Same(t, second, app.container.LLMModel())
}

func TestApp_AddLLM_NilIgnored(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	var typedNil *testLLM

	assert.NotPanics(t, func() {
		app.AddLLM(nil)
		app.AddLLM(typedNil)
	})
	assert.Nil(t, app.container.LLM(), "a nil or typed-nil model is not installed")
}

func TestApp_LLM_NilWhenNotAdded(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	assert.Nil(t, app.container.LLM())
}

func TestNew_AutoInitsLLMFromEnv(t *testing.T) {
	testutil.NewServerConfigs(t)
	t.Setenv("LLM_PROVIDER", "groq")
	t.Setenv("LLM_MODEL", "llama-3.3-70b-versatile")

	app := New()

	require.NotNil(t, app.container.LLM(), "an LLM configured via env is wired without app.AddLLM")

	client, ok := app.container.LLMModel().(*llm.Client)
	require.True(t, ok)
	assert.Equal(t, llm.Groq, client.Provider)
	assert.Equal(t, "llama-3.3-70b-versatile", client.Model)
}

func TestNew_NoLLMWhenProviderUnset(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	assert.Nil(t, app.container.LLM())
}

func TestNew_SkipsLLMWhenModelMissing(t *testing.T) {
	testutil.NewServerConfigs(t)
	t.Setenv("LLM_PROVIDER", "groq") // no LLM_MODEL

	app := New()

	assert.Nil(t, app.container.LLM(), "provider without a model is skipped, not a panic")
}
