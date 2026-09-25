// Package explain produce el texto que ve el operador (por qué es una anomalía y qué hacer).
//
// El motor (engine) decide tipo, severidad y prioridad; aquí solo se REDACTA a partir de esa evidencia
// (docs/DESIGN.md ADR 7). Hay dos implementaciones reales de Explainer: una plantilla determinista, que
// siempre funciona, y un LLM compatible con OpenAI. Si el LLM falla, se usa la plantilla.
package explain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/soypiipe/energy-ai/backend/internal/analysis/engine"
)

// Explanation es el texto para el operador.
type Explanation struct {
	Reason            string `json:"reason"`
	RecommendedAction string `json:"recommended_action"`
}

// Explainer redacta la explicación de una anomalía ya clasificada por el motor.
type Explainer interface {
	Explain(ctx context.Context, a engine.Anomaly) (Explanation, error)
}

// Límites de longitud: el texto se muestra en tablas y tarjetas; algo más largo es señal de que
// el LLM divagó. La plantilla también los cumple.
const (
	maxReasonLen = 600
	maxActionLen = 400
)

// Validate comprueba que la explicación sea usable antes de guardarla.
func (e Explanation) Validate() error {
	reason, action := strings.TrimSpace(e.Reason), strings.TrimSpace(e.RecommendedAction)
	switch {
	case reason == "":
		return errors.New("reason vacío")
	case action == "":
		return errors.New("recommended_action vacío")
	case utf8.RuneCountInString(reason) > maxReasonLen:
		return fmt.Errorf("reason demasiado largo (%d > %d)", utf8.RuneCountInString(reason), maxReasonLen)
	case utf8.RuneCountInString(action) > maxActionLen:
		return fmt.Errorf("recommended_action demasiado largo (%d > %d)", utf8.RuneCountInString(action), maxActionLen)
	}
	return nil
}
