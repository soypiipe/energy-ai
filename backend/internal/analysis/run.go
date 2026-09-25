// Package analysis orquesta el análisis: la cola de ejecuciones (analysis_runs), el worker que las
// procesa y la API HTTP. Los cálculos viven en el subpaquete engine (funciones puras).
package analysis

import (
	"encoding/json"
	"time"
)

// Estados de una ejecución. Coinciden con el CHECK de la tabla analysis_runs.
const (
	StatusPending   = "PENDING"
	StatusRunning   = "RUNNING"
	StatusCompleted = "COMPLETED"
	StatusFailed    = "FAILED"
)

// Run es una ejecución del análisis: una fila de la cola.
type Run struct {
	ID          string          `json:"id"`
	Status      string          `json:"status"`
	CurrentStep *string         `json:"current_step"`
	Error       *string         `json:"error"`
	Summary     json.RawMessage `json:"summary"`
	CreatedAt   time.Time       `json:"created_at"`
	StartedAt   *time.Time      `json:"started_at"`
	FinishedAt  *time.Time      `json:"finished_at"`
}
