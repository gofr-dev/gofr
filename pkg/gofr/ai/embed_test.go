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

// ctx.LLM().Embed forwards the input to the model, returns its vectors, and records exactly one
// successful call — no caller-side type assertion.
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
	l := NewLLM(em, Deps{Metrics: m})

	resp, err := l.Embed(t.Context(), []string{"hello", "world"})
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
	l := NewLLM(&fakeModel{}, Deps{Metrics: m})

	_, err := l.Embed(t.Context(), []string{"x"})
	require.ErrorIs(t, err, ErrEmbedNotSupported)
	require.Len(t, m.counters, 1)
	assert.Equal(t, statusError, labelValue(m.counters[0].labels, "status"))
}

// An error from the underlying model propagates and is recorded as an error call.
func TestLLM_Embed_ModelErrorRecordsError(t *testing.T) {
	m := &fakeMetrics{}
	em := &embedModel{fakeModel: &fakeModel{}, embErr: errProviderDown}
	l := NewLLM(em, Deps{Metrics: m})

	_, err := l.Embed(t.Context(), []string{"x"})
	require.ErrorIs(t, err, errProviderDown)
	require.Len(t, m.counters, 1)
	assert.Equal(t, statusError, labelValue(m.counters[0].labels, "status"))
}
