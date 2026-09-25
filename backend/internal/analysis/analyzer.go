package analysis

import (
	"context"
	"fmt"

	"github.com/soypiipe/energy-ai/backend/internal/analysis/engine"
)

// Pasos que se reportan mientras corre una ejecución (los ve el usuario como progreso).
const (
	StepLoading   = "loading_data"
	StepDetecting = "detecting_anomalies"
	StepSaving    = "saving_results"
)

// Analyzer une la base de datos con el motor puro: carga los datos, corre el motor y guarda el resultado.
type Analyzer struct {
	repo *Repository
}

func NewAnalyzer(repo *Repository) *Analyzer {
	return &Analyzer{repo: repo}
}

// Summary es el resumen que queda en analysis_runs.summary.
type Summary struct {
	MetersAnalyzed int            `json:"meters_analyzed"`
	Anomalies      int            `json:"anomalies"`
	ByType         map[string]int `json:"by_type"`
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

	if err := setStep(StepSaving); err != nil {
		return nil, err
	}
	drafts := make([]Draft, len(anomalies))
	summary := Summary{MetersAnalyzed: len(readings), Anomalies: len(anomalies), ByType: map[string]int{}}
	for i, an := range anomalies {
		drafts[i] = Draft{Anomaly: an} // el texto lo agrega el Explainer (Fase 3)
		summary.ByType[string(an.Type)]++
	}
	if err := a.repo.SaveAnomalies(ctx, run.ID, drafts); err != nil {
		return nil, err
	}
	return summary, nil
}
