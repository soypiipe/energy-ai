package explain

import (
	"context"
	"errors"
	"testing"

	"github.com/soypiipe/energy-ai/backend/internal/analysis/engine"
)

type stub struct {
	e   Explanation
	err error
	n   int
}

func (s *stub) Explain(context.Context, engine.Anomaly) (Explanation, error) {
	s.n++
	return s.e, s.err
}

func TestFallbackUsesPrimaryWhenItWorks(t *testing.T) {
	p := &stub{e: Explanation{Reason: "llm", RecommendedAction: "a", Source: SourceLLM}}
	f := &stub{e: Explanation{Reason: "tpl", RecommendedAction: "a", Source: SourceTemplate}}
	got, err := NewFallback(p, f).Explain(context.Background(), m109())
	if err != nil || got.Source != SourceLLM || f.n != 0 {
		t.Errorf("got %+v err %v, fallback llamado %d veces", got, err, f.n)
	}
}

func TestFallbackFallsBackOnError(t *testing.T) {
	p := &stub{err: errors.New("timeout")}
	got, err := NewFallback(p, NewTemplate()).Explain(context.Background(), m109())
	if err != nil || got.Source != SourceTemplate || got.Reason == "" {
		t.Errorf("got %+v err %v", got, err)
	}
}

func TestFallbackStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &stub{}
	if _, err := NewFallback(&stub{err: errors.New("x")}, f).Explain(ctx, m109()); err == nil || f.n != 0 {
		t.Errorf("con el contexto cancelado no debe usarse la plantilla: err=%v n=%d", err, f.n)
	}
}
