// Package dashboard arma los KPIs de la pantalla principal combinando medidores y anomalías.
package dashboard

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/soypiipe/energy-ai/backend/internal/analysis"
	"github.com/soypiipe/energy-ai/backend/internal/httpx"
	"github.com/soypiipe/energy-ai/backend/internal/meter"
)

type Summary struct {
	Meters       MeterCounts     `json:"meters"`
	Consumption  Consumption     `json:"consumption"`
	Anomalies    AnomalyCounts   `json:"anomalies"`
	TopPriority  *TopPriority    `json:"top_priority"`  // qué revisar primero; null si no hay anomalías
	LastAnalysis *LastAnalysisAt `json:"last_analysis"` // null si nunca se corrió un análisis
}

// MeterCounts cuenta medidores por estado operativo (OK / ALERT / CRITICAL).
type MeterCounts struct {
	Total    int `json:"total"`
	OK       int `json:"ok"`
	Alert    int `json:"alert"`
	Critical int `json:"critical"`
}

// Consumption compara las últimas 24 h del dataset contra el baseline diario, sumando todos los medidores.
type Consumption struct {
	TotalKWh     float64 `json:"total_kwh"`     // todo el periodo
	CurrentKWh   float64 `json:"current_kwh"`   // últimas 24 h
	BaselineKWh  float64 `json:"baseline_kwh"`  // promedio diario de los primeros 7 días
	VariationPct float64 `json:"variation_pct"` // (actual / baseline - 1) * 100
}

// AnomalyCounts cuenta las anomalías del último análisis.
type AnomalyCounts struct {
	Total         int            `json:"total"`
	Open          int            `json:"open"`
	HighPriority  int            `json:"high_priority"`  // severidad HIGH
	AvgConfidence float64        `json:"avg_confidence"` // confianza media de la IA (0-1); 0 si no hay anomalías
	BySeverity    map[string]int `json:"by_severity"`
	ByType        map[string]int `json:"by_type"`
}

type TopPriority struct {
	AnomalyID string `json:"anomaly_id"`
	MeterID   string `json:"meter_id"`
	Type      string `json:"type"`
	Severity  string `json:"severity"`
}

type LastAnalysisAt struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	FinishedAt *time.Time `json:"finished_at"`
}

// Build calcula los KPIs. Es una función pura: recibe los datos ya cargados, así se prueba sin base.
// anomalies debe venir ordenada por prioridad (como la devuelve el repositorio).
func Build(meters []meter.Summary, anomalies []analysis.Anomaly, last *analysis.Run) Summary {
	s := Summary{
		Anomalies: AnomalyCounts{BySeverity: map[string]int{}, ByType: map[string]int{}},
	}

	s.Meters.Total = len(meters)
	for _, m := range meters {
		switch m.Status {
		case meter.StatusCritical:
			s.Meters.Critical++
		case meter.StatusAlert:
			s.Meters.Alert++
		default:
			s.Meters.OK++
		}
		s.Consumption.TotalKWh += m.TotalKWh
		s.Consumption.CurrentKWh += m.CurrentKWh
		s.Consumption.BaselineKWh += m.BaselineKWh
	}
	s.Consumption.TotalKWh = round1(s.Consumption.TotalKWh)
	s.Consumption.CurrentKWh = round1(s.Consumption.CurrentKWh)
	s.Consumption.BaselineKWh = round1(s.Consumption.BaselineKWh)
	s.Consumption.VariationPct = meter.VariationPct(s.Consumption.CurrentKWh, s.Consumption.BaselineKWh)

	s.Anomalies.Total = len(anomalies)
	var confSum float64
	for _, a := range anomalies {
		if a.Status == analysis.AnomalyOpen {
			s.Anomalies.Open++
		}
		if a.Severity == "HIGH" {
			s.Anomalies.HighPriority++
		}
		confSum += a.Confidence
		s.Anomalies.BySeverity[a.Severity]++
		s.Anomalies.ByType[a.Type]++
	}
	if len(anomalies) > 0 {
		s.Anomalies.AvgConfidence = math.Round(confSum/float64(len(anomalies))*100) / 100
		top := anomalies[0]
		s.TopPriority = &TopPriority{AnomalyID: top.ID, MeterID: top.MeterID, Type: top.Type, Severity: top.Severity}
	}
	if last != nil {
		s.LastAnalysis = &LastAnalysisAt{ID: last.ID, Status: last.Status, FinishedAt: last.FinishedAt}
	}
	return s
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// Handler expone GET /dashboard/summary.
type Handler struct {
	meters   *meter.Repository
	analysis *analysis.Repository
}

func NewHandler(meters *meter.Repository, analysis *analysis.Repository) *Handler {
	return &Handler{meters: meters, analysis: analysis}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /dashboard/summary", h.summary)
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	s, err := h.load(r.Context())
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s)
}

func (h *Handler) load(ctx context.Context) (Summary, error) {
	meters, err := h.meters.ListSummaries(ctx)
	if err != nil {
		return Summary{}, fmt.Errorf("medidores: %w", err)
	}
	_, anomalies, err := h.analysis.ListAnomalies(ctx, analysis.AnomalyFilter{})
	if err != nil {
		return Summary{}, fmt.Errorf("anomalías: %w", err)
	}
	last, err := h.analysis.LatestCompletedRun(ctx)
	if err != nil {
		return Summary{}, fmt.Errorf("último análisis: %w", err)
	}
	return Build(meters, anomalies, last), nil
}
