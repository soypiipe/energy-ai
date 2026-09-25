package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/soypiipe/energy-ai/backend/internal/httpx"
)

var ErrAnomalyNotFound = errors.New("anomalía no encontrada")

// Estados de una anomalía (ciclo de vida del operador). Coinciden con el CHECK de la tabla.
const (
	AnomalyOpen         = "OPEN"
	AnomalyAcknowledged = "ACKNOWLEDGED"
	AnomalyResolved     = "RESOLVED"
)

func validAnomalyStatus(s string) bool {
	return s == AnomalyOpen || s == AnomalyAcknowledged || s == AnomalyResolved
}

// Anomaly es una anomalía tal como la ve la API.
type Anomaly struct {
	ID                string          `json:"id"`
	AnalysisID        string          `json:"analysis_id"`
	MeterID           string          `json:"meter_id"`
	MeterName         string          `json:"meter_name"`
	DetectedAt        httpx.LocalTime `json:"detected_at"`
	Type              string          `json:"type"`
	Severity          string          `json:"severity"`
	Confidence        float64         `json:"confidence"`
	PriorityScore     float64         `json:"priority_score"`
	Reason            string          `json:"reason"`
	RecommendedAction string          `json:"recommended_action"`
	Evidence          json.RawMessage `json:"evidence"`
	Status            string          `json:"status"`
	CreatedAt         time.Time       `json:"created_at"`
}

// AnomalyFilter son los filtros opcionales del listado; el valor vacío significa "sin filtro".
type AnomalyFilter struct {
	MeterID  string
	Type     string
	Severity string
	Status   string
}

const anomalyColumns = `a.id, a.analysis_id, a.meter_id, m.name, a.detected_at, a.type, a.severity,
	a.confidence, a.priority_score, a.reason, a.recommended_action, a.evidence, a.status, a.created_at`

func scanAnomaly(row pgx.Row) (Anomaly, error) {
	var a Anomaly
	var detected time.Time
	err := row.Scan(&a.ID, &a.AnalysisID, &a.MeterID, &a.MeterName, &detected, &a.Type, &a.Severity,
		&a.Confidence, &a.PriorityScore, &a.Reason, &a.RecommendedAction, &a.Evidence, &a.Status, &a.CreatedAt)
	a.DetectedAt = httpx.LocalTime(detected)
	return a, err
}

// ListAnomalies devuelve las anomalías del último análisis COMPLETED, la de mayor prioridad primero.
// Sin ningún análisis completado devuelve una lista vacía y analysisID vacío.
func (r *Repository) ListAnomalies(ctx context.Context, f AnomalyFilter) (analysisID string, list []Anomaly, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT id FROM analysis_runs WHERE status = 'COMPLETED' ORDER BY finished_at DESC LIMIT 1`,
	).Scan(&analysisID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", []Anomaly{}, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("buscar último análisis: %w", err)
	}

	// Filtros parametrizados: "$n::text IS NULL OR col = $n" evita armar SQL con strings.
	rows, err := r.pool.Query(ctx, `
		SELECT `+anomalyColumns+`
		FROM anomalies a JOIN meters m ON m.meter_id = a.meter_id
		WHERE a.analysis_id = $1
		  AND ($2::text IS NULL OR a.meter_id = $2)
		  AND ($3::text IS NULL OR a.type = $3)
		  AND ($4::text IS NULL OR a.severity = $4)
		  AND ($5::text IS NULL OR a.status = $5)
		ORDER BY a.priority_score DESC, a.meter_id`,
		analysisID, nullable(f.MeterID), nullable(f.Type), nullable(f.Severity), nullable(f.Status))
	if err != nil {
		return "", nil, fmt.Errorf("consultar anomalías: %w", err)
	}
	list, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (Anomaly, error) { return scanAnomaly(row) })
	if err != nil {
		return "", nil, fmt.Errorf("leer anomalías: %w", err)
	}
	if list == nil {
		list = []Anomaly{}
	}
	return analysisID, list, nil
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GetAnomaly devuelve una anomalía por id (de cualquier análisis).
func (r *Repository) GetAnomaly(ctx context.Context, id string) (Anomaly, error) {
	a, err := scanAnomaly(r.pool.QueryRow(ctx,
		`SELECT `+anomalyColumns+` FROM anomalies a JOIN meters m ON m.meter_id = a.meter_id WHERE a.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Anomaly{}, ErrAnomalyNotFound
	}
	if err != nil {
		return Anomaly{}, fmt.Errorf("consultar anomalía: %w", err)
	}
	return a, nil
}

// UpdateAnomalyStatus cambia el estado de una anomalía y devuelve la versión actualizada.
func (r *Repository) UpdateAnomalyStatus(ctx context.Context, id, status string) (Anomaly, error) {
	if !validAnomalyStatus(status) {
		return Anomaly{}, fmt.Errorf("estado inválido %q", status)
	}
	tag, err := r.pool.Exec(ctx, `UPDATE anomalies SET status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return Anomaly{}, fmt.Errorf("actualizar anomalía: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Anomaly{}, ErrAnomalyNotFound
	}
	return r.GetAnomaly(ctx, id)
}
