package ai

import (
	"context"
	"encoding/json"
	"sync"

	"gofr.dev/pkg/gofr/datasource"
)

// NewLLM wraps a provider Model with GoFr's instrumentation and tool access, returning the LLM
// surface handlers use via ctx.LLM(). It returns nil if m is nil.
func NewLLM(m Model, d Deps) LLM {
	if m == nil {
		return nil
	}

	provider, model := m.Name(), m.Name()
	if desc, ok := m.(Descriptor); ok {
		provider, model = desc.ProviderName(), desc.ModelName()
	}

	return &llm{model: m, deps: d, providerLabel: provider, modelLabel: model}
}

type llm struct {
	model         Model
	deps          Deps
	providerLabel string
	modelLabel    string
}

func (l *llm) Name() string { return l.model.Name() }

func (l *llm) HealthCheck(ctx context.Context) datasource.Health { return l.model.HealthCheck(ctx) }

func (l *llm) Generate(ctx context.Context, prompt string, opts ...Option) (*Response, error) {
	return l.call(ctx, opGenerate, func(ctx context.Context) (*Response, error) {
		return l.model.Chat(ctx, []Message{{Role: RoleUser, Content: prompt}}, opts...)
	})
}

func (l *llm) Chat(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	return l.call(ctx, opChat, func(ctx context.Context) (*Response, error) {
		return l.model.Chat(ctx, messages, opts...)
	})
}

func (l *llm) Stream(ctx context.Context, messages []Message, opts ...Option) (Streamer, error) {
	rec := StartCall(ctx, &CallInfo{Deps: l.deps, Provider: l.providerLabel, Model: l.modelLabel, Op: opStream})

	sm, ok := l.model.(StreamingModel)
	if !ok {
		rec.Finish(Usage{}, ErrStreamNotSupported)
		return nil, ErrStreamNotSupported
	}

	s, err := sm.Stream(rec.Context(), messages, opts...)
	if err != nil {
		rec.Finish(Usage{}, err)
		return nil, err
	}

	return &instrumentedStream{Streamer: s, rec: rec}, nil
}

func (l *llm) Tools() Tools {
	if l.deps.Tools != nil {
		return l.deps.Tools
	}

	return emptyTools{}
}

func (l *llm) call(ctx context.Context, op string, fn func(context.Context) (*Response, error)) (*Response, error) {
	return Instrument(ctx, &CallInfo{Deps: l.deps, Provider: l.providerLabel, Model: l.modelLabel, Op: op}, fn)
}

// instrumentedStream closes the call's Recorder exactly once, when the stream is exhausted or
// closed. A provider stream may implement usageReporter to supply token counts at that point.
type instrumentedStream struct {
	Streamer

	rec  *Recorder
	once sync.Once
}

type usageReporter interface{ Usage() Usage }

func (s *instrumentedStream) Next() (any, bool) {
	v, ok := s.Streamer.Next()
	if !ok {
		s.finish()
	}

	return v, ok
}

func (s *instrumentedStream) Close() error {
	err := s.Streamer.Close()
	s.finish()

	return err
}

func (s *instrumentedStream) finish() {
	s.once.Do(func() {
		var u Usage
		if r, ok := s.Streamer.(usageReporter); ok {
			u = r.Usage()
		}

		s.rec.Finish(u, s.Streamer.Err())
	})
}

type emptyTools struct{}

func (emptyTools) List() []ToolSpec { return nil }

func (emptyTools) Only(...string) Tools { return emptyTools{} }

func (emptyTools) Call(context.Context, string, json.RawMessage) (Result, error) {
	return Result{}, ErrToolNotFound
}
