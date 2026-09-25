package analysis

import (
	"context"
	"fmt"
	"sync"

	"github.com/soypiipe/energy-ai/backend/internal/analysis/engine"
	"github.com/soypiipe/energy-ai/backend/internal/explain"
)

// Pasos que se reportan mientras corre una ejecución (los ve el usuario como progreso).
const (
	StepLoading    = "loading_data"
	StepDetecting  = "detecting_anomalies"
	StepExplaining = "explaining"
	StepSaving     = "saving_results"
)

// Analyzer une la base de datos con el motor puro: carga los datos, corre el motor y guarda el resultado.
type Analyzer struct {
	repo      *Repository
	explainer explain.Explainer
}

func NewAnalyzer(repo *Repository, explainer explain.Explainer) *Analyzer {
	return &Analyzer{repo: repo, explainer: explainer}
}

// Summary es el resumen que queda en analysis_runs.summary.
type Summary struct {
	MetersAnalyzed int            `json:"meters_analyzed"`
	Anomalies      int            `json:"anomalies"`
	ByType         map[string]int `json:"by_type"`
	ExplainedBy    map[string]int `json:"explained_by"` // cuántas explicaciones redactó cada fuente (llm / template)
}

// Process es la ProcessFunc del worker.
func (a *Analyzer) Process(ctx context.Context, run Run, setStep func(string) error) (any, error) {
	if err := setStep(StepLoading); err != nil {
		return nil, err
	}
	readings, events, err := a.repo.LoadDataset(ctx)
	if err != nil {
		return nil, fmt.Errorf("cargar datos: %w", err)
	}

	if err := setStep(StepDetecting); err != nil {
		return nil, err
	}
	anomalies, err := engine.Analyze(readings, events)
	if err != nil {
		return nil, fmt.Errorf("motor de análisis: %w", err)
	}

	if err := setStep(StepExplaining); err != nil {
		return nil, err
	}
	drafts, err := a.explainAll(ctx, anomalies)
	if err != nil {
		return nil, err
	}

	if err := setStep(StepSaving); err != nil {
		return nil, err
	}
	summary := Summary{
		MetersAnalyzed: len(readings), Anomalies: len(anomalies),
		ByType: map[string]int{}, ExplainedBy: map[string]int{},
	}
	for _, d := range drafts {
		summary.ByType[string(d.Anomaly.Type)]++
		summary.ExplainedBy[d.Source]++
	}
	if err := a.repo.SaveAnomalies(ctx, run.ID, drafts); err != nil {
		return nil, err
	}
	return summary, nil
}

// explainAll redacta la explicación de cada anomalía. Se hace en paralelo: si el LLM está lento, el
// análisis tarda como una sola llamada y no como la suma de todas. El resultado conserva el orden.
func (a *Analyzer) explainAll(ctx context.Context, anomalies []engine.Anomaly) ([]Draft, error) {
	drafts := make([]Draft, len(anomalies))
	errs := make([]error, len(anomalies))
	var wg sync.WaitGroup
	for i, an := range anomalies {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e, err := a.explainer.Explain(ctx, an)
			if err != nil {
				errs[i] = fmt.Errorf("explicar anomalía de %s: %w", an.MeterID, err)
				return
			}
			drafts[i] = Draft{Anomaly: an, Reason: e.Reason, RecommendedAction: e.RecommendedAction, Source: e.Source}
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return drafts, nil
}
