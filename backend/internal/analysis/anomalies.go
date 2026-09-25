package analysis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/soypiipe/energy-ai/backend/internal/analysis/engine"
)

// localLayout: los timestamps son hora local de planta sin zona (docs/DESIGN.md §2), así que en el
// JSON de evidencia se escriben sin "Z" para no afirmar que son UTC.
const localLayout = "2006-01-02T15:04:05"

// Draft es una anomalía del motor junto con el texto para el operador que produce el Explainer.
type Draft struct {
	Anomaly           engine.Anomaly
	Reason            string
	RecommendedAction string
	Source            string // quién redactó: explain.SourceLLM o explain.SourceTemplate
}

// evidenceDoc es el JSON que se guarda en anomalies.evidence: todo lo que respalda la anomalía
// (hallazgos con sus métricas y el evento relacionado). Es lo que ve la pantalla de investigación.
type evidenceDoc struct {
	Findings          []findingDoc `json:"findings"`
	RelatedEvent      *eventDoc    `json:"related_event"`
	ExplanationSource string       `json:"explanation_source"` // "llm" o "template": la UI puede indicar si lo redactó IA
}

type findingDoc struct {
	Kind     engine.FindingKind `json:"kind"`
	Start    string             `json:"start"`
	End      *string            `json:"end"` // null si sigue activo al final de los datos
	Evidence []engine.Evidence  `json:"evidence"`
}

type eventDoc struct {
	Timestamp   string           `json:"timestamp"`
	Type        engine.EventType `json:"type"`
	Description string           `json:"description"`
}

func buildEvidence(a engine.Anomaly, source string) ([]byte, error) {
	doc := evidenceDoc{Findings: make([]findingDoc, 0, len(a.Findings)), ExplanationSource: source}
	for _, f := range a.Findings {
		fd := findingDoc{Kind: f.Kind, Start: f.Start.Format(localLayout), Evidence: f.Evidence}
		if !f.End.IsZero() {
			end := f.End.Format(localLayout)
			fd.End = &end
		}
		doc.Findings = append(doc.Findings, fd)
	}
	if e := a.RelatedEvent; e != nil {
		doc.RelatedEvent = &eventDoc{Timestamp: e.Timestamp.Format(localLayout), Type: e.Type, Description: e.Description}
	}
	return json.Marshal(doc)
}

// SaveAnomalies guarda las anomalías de una ejecución en una sola transacción. Es idempotente:
// borra primero lo que hubiera de esa ejecución, así reintentar tras un fallo no duplica filas.
func (r *Repository) SaveAnomalies(ctx context.Context, runID string, drafts []Draft) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacción: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // tras Commit es un no-op

	if _, err := tx.Exec(ctx, `DELETE FROM anomalies WHERE analysis_id = $1`, runID); err != nil {
		return fmt.Errorf("limpiar anomalías previas: %w", err)
	}
	for _, d := range drafts {
		a := d.Anomaly
		evidence, err := buildEvidence(a, d.Source)
		if err != nil {
			return fmt.Errorf("serializar evidencia de %s: %w", a.MeterID, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO anomalies (analysis_id, meter_id, detected_at, type, severity, confidence,
			                       priority_score, reason, recommended_action, evidence)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			runID, a.MeterID, a.DetectedAt, string(a.Type), string(a.Severity), a.Confidence,
			a.PriorityScore, d.Reason, d.RecommendedAction, evidence); err != nil {
			return fmt.Errorf("guardar anomalía de %s: %w", a.MeterID, err)
		}
	}
	return tx.Commit(ctx)
}

// LoadDataset lee todas las lecturas y eventos, agrupados por medidor, en el formato del motor.
func (r *Repository) LoadDataset(ctx context.Context) (map[string][]engine.Reading, map[string][]engine.Event, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT meter_id, ts, consumption_kwh, voltage_v, current_a, power_factor
		FROM readings ORDER BY meter_id, ts`)
	if err != nil {
		return nil, nil, fmt.Errorf("consultar lecturas: %w", err)
	}
	defer rows.Close()
	readings := map[string][]engine.Reading{}
	for rows.Next() {
		var id string
		var rd engine.Reading
		if err := rows.Scan(&id, &rd.Timestamp, &rd.ConsumptionKWh, &rd.VoltageV, &rd.CurrentA, &rd.PowerFactor); err != nil {
			return nil, nil, fmt.Errorf("leer lectura: %w", err)
		}
		readings[id] = append(readings[id], rd)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("leer lecturas: %w", err)
	}

	evRows, err := r.pool.Query(ctx, `SELECT meter_id, ts, type, description FROM events ORDER BY meter_id, ts`)
	if err != nil {
		return nil, nil, fmt.Errorf("consultar eventos: %w", err)
	}
	defer evRows.Close()
	events := map[string][]engine.Event{}
	for evRows.Next() {
		var id, typ string
		var e engine.Event
		if err := evRows.Scan(&id, &e.Timestamp, &typ, &e.Description); err != nil {
			return nil, nil, fmt.Errorf("leer evento: %w", err)
		}
		e.Type = engine.EventType(typ)
		events[id] = append(events[id], e)
	}
	if err := evRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("leer eventos: %w", err)
	}
	return readings, events, nil
}
