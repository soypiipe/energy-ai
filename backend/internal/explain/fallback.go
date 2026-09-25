package explain

import (
	"context"
	"log/slog"

	"github.com/soypiipe/energy-ai/backend/internal/analysis/engine"
)

// Fallback prueba primary y, si falla (red, timeout, respuesta inválida, cuota), usa fallback.
// Así el análisis nunca depende de que el LLM esté disponible: la demo funciona con o sin clave.
type Fallback struct {
	primary  Explainer
	fallback Explainer
}

func NewFallback(primary, fallback Explainer) *Fallback {
	return &Fallback{primary: primary, fallback: fallback}
}

func (f *Fallback) Explain(ctx context.Context, a engine.Anomaly) (Explanation, error) {
	e, err := f.primary.Explain(ctx, a)
	if err == nil {
		return e, nil
	}
	if ctx.Err() != nil {
		return Explanation{}, ctx.Err() // se canceló el análisis: no tiene sentido seguir
	}
	slog.Warn("explicación con LLM falló; se usa la plantilla", "meter_id", a.MeterID, "err", err)
	return f.fallback.Explain(ctx, a)
}
