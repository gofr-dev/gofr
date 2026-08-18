package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/datasource"
)

// LLM documents a compatibility freeze: capabilities arrive as new optional interfaces asserted by
// the caller, never as new methods on LLM, so that hand-written fakes and third-party wrappers keep
// compiling across minor versions. The freeze is only worth the words if something enforces it.
//
// legacyFake is the fake a user writes against LLM. It deliberately implements the interface and
// nothing more — no Embed, no future capability method. If a capability is ever added to LLM
// directly, this file stops compiling, which is the point: the break surfaces here, in this repo,
// instead of in a user's build after they upgrade.
type legacyFake struct{}

func (*legacyFake) Chat(context.Context, []Message, ...Option) (*Response, error) {
	return &Response{Content: "canned"}, nil
}

func (*legacyFake) Generate(context.Context, string, ...Option) (*Response, error) {
	return &Response{Content: "canned"}, nil
}

func (*legacyFake) Stream(context.Context, []Message, ...Option) (Streamer, error) {
	return nil, ErrStreamNotSupported
}

func (*legacyFake) Tools() Tools                                  { return emptyTools{} }
func (*legacyFake) HealthCheck(context.Context) datasource.Health { return datasource.Health{} }
func (*legacyFake) Name() string                                  { return "legacy-fake" }

// The assignment a user actually writes in their tests.
var _ LLM = (*legacyFake)(nil)

// The capability is reachable from the LLM GoFr hands to a handler, without being declared on LLM.
var _ EmbeddingLLM = (*llm)(nil)

func TestLLM_FreezeHoldsForHandWrittenFakes(t *testing.T) {
	// Compiling is the assertion; this body just proves the fake is usable as an LLM.
	var l LLM = &legacyFake{}

	resp, err := l.Chat(t.Context(), []Message{{Role: RoleUser, Content: "hi"}})
	require.NoError(t, err)
	assert.Equal(t, "canned", resp.Content)

	_, err = l.Stream(t.Context(), nil)
	require.ErrorIs(t, err, ErrStreamNotSupported)
}

func TestLLM_EmbeddingCapabilityAlwaysAssertable(t *testing.T) {
	// Chat-only provider: the assertion must still succeed, so a handler never has to distinguish
	// "capability missing from this build" from "provider does not support it".
	e, ok := NewLLM(&fakeModel{}, Deps{}).(EmbeddingLLM)
	require.True(t, ok, "ctx.LLM() must always be assertable to EmbeddingLLM")

	_, err := e.Embed(t.Context(), []string{"x"})
	assert.ErrorIs(t, err, ErrEmbedNotSupported, "a chat-only provider reports the error, not a failed assertion")
}

// nilEmbedder is a third-party provider that violates the contract mildly: it reports success but
// returns no response. The Chat path already tolerates this (record skips a nil response), so Embed
// must too rather than panicking the handler.
type nilEmbedder struct{ *fakeModel }

func (nilEmbedder) Embed(context.Context, []string, ...Option) (*EmbeddingResponse, error) {
	return nil, nil //nolint:nilnil // deliberately models a misbehaving third-party provider
}

func TestLLM_Embed_NilResponseFromProviderDoesNotPanic(t *testing.T) {
	m := &fakeMetrics{}
	e := embedderOf(t, nilEmbedder{fakeModel: &fakeModel{}}, Deps{Metrics: m})

	assert.NotPanics(t, func() {
		resp, err := e.Embed(t.Context(), []string{"x"})
		require.NoError(t, err)
		assert.Nil(t, resp)
	})

	// The call is still recorded, with zero usage rather than a crash.
	require.Len(t, m.counters, 1)
	assert.Equal(t, statusSuccess, labelValue(m.counters[0].labels, "status"))
}

// Chat's tolerance of the same violation, pinned alongside Embed's so the two paths cannot drift.
type nilChatModel struct{ *fakeModel }

func (nilChatModel) Chat(context.Context, []Message, ...Option) (*Response, error) {
	return nil, nil //nolint:nilnil // deliberately models a misbehaving third-party provider
}

func TestLLM_Chat_NilResponseFromProviderDoesNotPanic(t *testing.T) {
	l := NewLLM(nilChatModel{fakeModel: &fakeModel{}}, Deps{})

	assert.NotPanics(t, func() {
		resp, err := l.Chat(t.Context(), []Message{{Role: RoleUser, Content: "hi"}})
		require.NoError(t, err)
		assert.Nil(t, resp)
	})
}
