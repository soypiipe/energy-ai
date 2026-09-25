package engine

import (
	"math"
	"time"
)

// Confianza y prioridad son fórmulas deterministas y auditables; no las decide el LLM.
//
// Confianza (0.50–0.99): cuánta certeza hay en la clasificación.
//
//		confianza = 0.50 + 0.20·magnitud + 0.15·corroboración + 0.15·respaldo
//
//	  - magnitud (0–1): |cambio %| del hallazgo principal / 100, con tope 1. En calidad de datos,
//	    lecturas marcadas / 24, con tope 1.
//	  - corroboración (0–1): tipos de hallazgo distintos además del principal / 2, con tope 1
//	    (consumo + corriente + FP que se refuerzan entre sí).
//	  - respaldo (0–1): según el tipo de anomalía.
//	    · EXPLAINABLE y FALSE_POSITIVE: 1.0 si el evento cae a ±30 min del inicio, 0.7 si está en ±2 h.
//	    · REAL y DATA_QUALITY: 1.0 si el problema sigue activo al final de los datos, 0.5 si ya terminó.
//
// Prioridad (mayor = revisar primero):
//
//	prioridad = peso_severidad · (0.5 + 0.5·magnitud) · confianza
//
// con pesos LOW=1, MEDIUM=2, HIGH=3. La magnitud entra con piso de 0.5 para que la severidad mande
// y la magnitud desempate entre anomalías de la misma severidad.
const (
	baseConfidence = 0.50
	maxConfidence  = 0.99
	closeEvent     = 30 * time.Minute
	dqFullFlagged  = 24.0 // lecturas marcadas que equivalen a magnitud 1
)

func score(a *Anomaly, primary Finding) (confidence, priority float64) {
	mag := magnitudeScore(a, primary)
	kinds := map[FindingKind]bool{}
	for _, f := range a.Findings {
		kinds[f.Kind] = true
	}
	corroboration := math.Min(1, float64(len(kinds)-1)/2)

	confidence = baseConfidence + 0.20*mag + 0.15*corroboration + 0.15*support(a, primary)
	confidence = round(math.Min(confidence, maxConfidence), 2)
	priority = round(float64(a.Severity.Rank())*(0.5+0.5*mag)*confidence, 2)
	return confidence, priority
}

func magnitudeScore(a *Anomaly, primary Finding) float64 {
	if a.Type == AnomalyDataQuality {
		for _, e := range primary.Evidence {
			if e.Metric == "flagged_readings" {
				return math.Min(1, e.Observed/dqFullFlagged)
			}
		}
		return 0
	}
	return math.Min(1, magnitude(primary)/100)
}

func support(a *Anomaly, primary Finding) float64 {
	switch a.Type {
	case AnomalyExplainable, AnomalyFalsePos:
		if a.RelatedEvent != nil && abs64(a.RelatedEvent.Timestamp.Sub(primary.Start)) <= closeEvent {
			return 1
		}
		return 0.7
	default:
		if primary.End.IsZero() {
			return 1
		}
		return 0.5
	}
}
