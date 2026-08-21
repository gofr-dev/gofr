package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errProviderDown = errors.New("provider down")

// embedModel adds the optional Embedder capability to fakeModel.
type embedModel struct {
	*fakeModel
	embResp  *EmbeddingResponse
	embErr   error
	gotInput []string
}

func (e *embedModel) Embed(_ context.Context, input []string, _ ...Option) (*EmbeddingResponse, error) {
	e.gotInput = input
	return e.embResp, e.embErr
}

// embedderOf builds the LLM GoFr hands to a handler and asserts EmbeddingLLM on it, exactly as a
// handler does. The assertion is part of what is under test: the LLM returned by ctx.LLM() must
// always satisfy EmbeddingLLM, so the capability is reachable without the caller knowing whether
// the configured provider happens to support it.
func embedderOf(t *testing.T, m Model, d Deps) EmbeddingLLM {
	t.Helper()

	e, ok := NewLLM(m, d).(EmbeddingLLM)
	require.True(t, ok, "the LLM returned by GoFr must always implement EmbeddingLLM")

	return e
}

// Embed forwards the input to the model, returns its vectors, and records exactly one successful
// call.
func TestLLM_Embed_Delegates(t *testing.T) {
	m := &fakeMetrics{}
	em := &embedModel{
		fakeModel: &fakeModel{},
		embResp: &EmbeddingResponse{
			Embeddings: [][]float32{{0.1, 0.2}, {0.3, 0.4}},
			Usage:      Usage{PromptTokens: 6},
			Model:      "embed-model",
		},
	}
	e := embedderOf(t, em, Deps{Metrics: m})

	resp, err := e.Embed(t.Context(), []string{"hello", "world"})
	require.NoError(t, err)
	assert.Equal(t, [][]float32{{0.1, 0.2}, {0.3, 0.4}}, resp.Embeddings)
	assert.Equal(t, "embed-model", resp.Model)
	assert.Equal(t, []string{"hello", "world"}, em.gotInput)

	require.Len(t, m.counters, 1)
	assert.Equal(t, statusSuccess, labelValue(m.counters[0].labels, "status"))
	assert.Equal(t, opEmbed, labelValue(m.counters[0].labels, "operation"))
}

// A chat-only model (no Embedder) returns ErrEmbedNotSupported gracefully — no panic — recorded as
// an error call, mirroring the Stream unsupported path.
func TestLLM_Embed_UnsupportedRecordsError(t *testing.T) {
	m := &fakeMetrics{}
	e := embedderOf(t, &fakeModel{}, Deps{Metrics: m})

	_, err := e.Embed(t.Context(), []string{"x"})
	require.ErrorIs(t, err, ErrEmbedNotSupported)
	require.Len(t, m.counters, 1)
	assert.Equal(t, statusError, labelValue(m.counters[0].labels, "status"))
}

// An error from the underlying model propagates and is recorded as an error call.
func TestLLM_Embed_ModelErrorRecordsError(t *testing.T) {
	m := &fakeMetrics{}
	em := &embedModel{fakeModel: &fakeModel{}, embErr: errProviderDown}
	e := embedderOf(t, em, Deps{Metrics: m})

	_, err := e.Embed(t.Context(), []string{"x"})
	require.ErrorIs(t, err, errProviderDown)
	require.Len(t, m.counters, 1)
	assert.Equal(t, statusError, labelValue(m.counters[0].labels, "status"))
}
