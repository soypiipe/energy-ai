package engine

import (
	"fmt"
	"sort"
	"time"
)

// electricalThresholds es el cambio relativo sostenido (≥24 h) que se considera eléctricamente relevante.
// Se calibraron sobre el ruido normal de los datos: FP ±4–5%, corriente ±8%, voltaje ±1.5%.
var electricalThresholds = []struct {
	metric    Metric
	threshold float64
}{
	{MetricPowerFactor, 0.08},
	{MetricCurrent, 0.25},
	{MetricVoltage, 0.02},
}

// DetectElectricalChanges busca cambios de nivel sostenidos en factor de potencia, corriente o voltaje.
//
// Reusa la misma lógica del cambio persistente sobre cada magnitud, con un umbral propio según su ruido.
// Los cambios de varias magnitudes que se solapan en el tiempo se unen en un solo hallazgo con toda la
// evidencia (ej. corriente ×2 y FP a la baja en M-109 son un mismo problema, no dos).
func DetectElectricalChanges(readings []Reading) ([]Finding, error) {
	var all []Finding
	for _, e := range electricalThresholds {
		b, err := BuildBaseline(readings, e.metric)
		if err != nil {
			return nil, fmt.Errorf("baseline de %s: %w", e.metric, err)
		}
		all = append(all, detectShifts(readings, b, e.threshold, FindingElectricalChange)...)
	}
	return mergeOverlapping(all), nil
}

// mergeOverlapping une hallazgos cuyo intervalo se solapa. End cero significa "sigue activo".
func mergeOverlapping(fs []Finding) []Finding {
	if len(fs) == 0 {
		return nil
	}
	sort.SliceStable(fs, func(i, j int) bool { return fs[i].Start.Before(fs[j].Start) })
	out := []Finding{fs[0]}
	for _, f := range fs[1:] {
		last := &out[len(out)-1]
		if !overlaps(*last, f) {
			out = append(out, f)
			continue
		}
		last.Evidence = append(last.Evidence, f.Evidence...)
		if last.End.IsZero() || f.End.IsZero() {
			last.End = time.Time{}
		} else if f.End.After(last.End) {
			last.End = f.End
		}
	}
	return out
}

// overlaps: f empieza en o antes del fin de a (fs viene ordenado por inicio).
func overlaps(a, f Finding) bool {
	return a.End.IsZero() || !f.Start.After(a.End)
}
