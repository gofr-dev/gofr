package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource"
)

type fakeModel struct {
	chatMsgs  []Message
	resp      *Response
	err       error
	stream    Streamer
	provider  string
	modelName string
}

func (f *fakeModel) Chat(_ context.Context, msgs []Message, _ ...Option) (*Response, error) {
	f.chatMsgs = msgs
	return f.resp, f.err
}

func (*fakeModel) HealthCheck(context.Context) datasource.Health {
	return datasource.Health{Status: datasource.StatusUp}
}

func (*fakeModel) Name() string { return "fake" }

type streamModel struct{ *fakeModel }

func (s streamModel) Stream(context.Context, []Message, ...Option) (Streamer, error) {
	return s.stream, s.err
}

type descModel struct{ *fakeModel }

func (d descModel) ProviderName() string { return d.provider }
func (d descModel) ModelName() string    { return d.modelName }

func TestNewLLM_Nil(t *testing.T) {
	assert.Nil(t, NewLLM(nil, Deps{}))
}

func TestLLM_Generate_BuildsSingleUserMessage(t *testing.T) {
	fm := &fakeModel{resp: &Response{Content: "ok"}}
	l := NewLLM(fm, Deps{})

	resp, err := l.Generate(t.Context(), "hello")

	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Content)
	assert.Equal(t, []Message{{Role: "user", Content: "hello"}}, fm.chatMsgs)
}

func TestLLM_Chat_Delegates(t *testing.T) {
	fm := &fakeModel{resp: &Response{Content: "ok"}}
	l := NewLLM(fm, Deps{})
	msgs := []Message{{Role: "system", Content: "s"}, {Role: "user", Content: "u"}}

	_, err := l.Chat(t.Context(), msgs)

	require.NoError(t, err)
	assert.Equal(t, msgs, fm.chatMsgs)
}

type fakeStreamer struct {
	items     []any
	idx       int
	err       error
	closed    bool
	usage     Usage
	toolCalls []ToolCall
}

func (f *fakeStreamer) Next() (any, bool) {
	if f.idx >= len(f.items) {
		return nil, false
	}

	v := f.items[f.idx]
	f.idx++

	return v, true
}

func (f *fakeStreamer) Err() error            { return f.err }
func (f *fakeStreamer) Close() error          { f.closed = true; return nil }
func (f *fakeStreamer) Usage() Usage          { return f.usage }
func (f *fakeStreamer) ToolCalls() []ToolCall { return f.toolCalls }

// The instrumented wrapper returned by ctx.LLM().Stream must forward assembled tool calls from the
// provider stream.
func TestLLM_Stream_ForwardsToolCalls(t *testing.T) {
	fs := &fakeStreamer{toolCalls: []ToolCall{{ID: "c1", Name: "search"}}}
	l := NewLLM(streamModel{&fakeModel{stream: fs}}, Deps{})

	s, err := l.Stream(t.Context(), nil)
	require.NoError(t, err)

	tc, ok := s.(ToolCallStreamer)
	require.True(t, ok, "the stream must expose ToolCallStreamer")
	require.Len(t, tc.ToolCalls(), 1)
	assert.Equal(t, "search", tc.ToolCalls()[0].Name)
}

func TestLLM_Stream_UnsupportedRecordsError(t *testing.T) {
	m := &fakeMetrics{}
	l := NewLLM(&fakeModel{}, Deps{Metrics: m})

	_, err := l.Stream(t.Context(), nil)

	require.ErrorIs(t, err, ErrStreamNotSupported)
	require.Len(t, m.counters, 1)
	assert.Equal(t, statusError, labelValue(m.counters[0].labels, "status"))
}

func TestLLM_Stream_InstrumentedFinishesOnce(t *testing.T) {
	m := &fakeMetrics{}
	fs := &fakeStreamer{items: []any{"a", "b"}, usage: Usage{CompletionTokens: 3}}
	l := NewLLM(streamModel{&fakeModel{stream: fs}}, Deps{Metrics: m})

	s, err := l.Stream(t.Context(), nil)
	require.NoError(t, err)

	var got []any

	for {
		v, ok := s.Next()
		if !ok {
			break
		}

		got = append(got, v)
	}

	require.NoError(t, s.Close()) // exhaustion + Close must still record exactly once

	assert.Equal(t, []any{"a", "b"}, got)
	assert.True(t, fs.closed)
	require.Len(t, m.counters, 1, "the stream call is recorded exactly once")
	assert.Equal(t, statusSuccess, labelValue(m.counters[0].labels, "status"))
	require.Len(t, m.histograms, 1, "completion tokens recorded once at finish")
}

func TestLLM_Tools_EmptyByDefault(t *testing.T) {
	l := NewLLM(&fakeModel{}, Deps{})
	tools := l.Tools()

	assert.Nil(t, tools.List())
	assert.Equal(t, tools, tools.Only("x"))

	_, err := tools.Call(t.Context(), "missing", nil)
	require.ErrorIs(t, err, ErrToolNotFound)
}

func TestLLM_Tools_UsesDeps(t *testing.T) {
	ctrl := gomock.NewController(t)
	want := NewMockTools(ctrl)
	l := NewLLM(&fakeModel{}, Deps{Tools: want})

	assert.Equal(t, want, l.Tools())
}

func TestLLM_Descriptor_LabelsCall(t *testing.T) {
	m := &fakeMetrics{}
	fm := &fakeModel{resp: &Response{}, provider: "openai", modelName: "gpt-4"}
	l := NewLLM(descModel{fm}, Deps{Metrics: m})

	_, err := l.Chat(t.Context(), nil)
	require.NoError(t, err)

	require.Len(t, m.counters, 1)
	assert.Equal(t, "openai", labelValue(m.counters[0].labels, "provider"))
	assert.Equal(t, "gpt-4", labelValue(m.counters[0].labels, "model"))
}

// The prompt must never leak into a metric label.
func TestLLM_NoSecretInLabels(t *testing.T) {
	const secret = "my-secret-prompt-and-key"

	m := &fakeMetrics{}
	fm := &fakeModel{resp: &Response{Usage: Usage{PromptTokens: 5}}}
	l := NewLLM(fm, Deps{Metrics: m})

	_, err := l.Generate(t.Context(), secret)
	require.NoError(t, err)

	for _, c := range append(m.counters, m.histograms...) {
		for _, label := range c.labels {
			assert.NotContains(t, label, secret)
		}
	}
}

func TestMockModel_SatisfiesInterfaces(t *testing.T) {
	ctrl := gomock.NewController(t)

	var (
		_ Model          = NewMockModel(ctrl)
		_ LLM            = NewMockLLM(ctrl)
		_ StreamingModel = NewMockStreamingModel(ctrl)
		// MockLLM stands in for the real *llm wrapper in handler tests, so it has to satisfy the
		// optional capabilities a handler asserts on the LLM as well, not just LLM itself.
		_ EmbeddingLLM = NewMockLLM(ctrl)
	)
}
