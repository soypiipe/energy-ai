package engine

import (
	"fmt"
	"sort"
)

// AnalyzeMeter corre todos los detectores sobre un medidor y clasifica el resultado.
// Devuelve nil si el medidor no tiene anomalías.
func AnalyzeMeter(meterID string, readings []Reading, events []Event) (*Anomaly, error) {
	rs := sortReadings(readings)
	base, err := BuildBaseline(rs, MetricConsumption)
	if err != nil {
		return nil, fmt.Errorf("baseline de consumo: %w", err)
	}
	electrical, err := DetectElectricalChanges(rs)
	if err != nil {
		return nil, fmt.Errorf("cambios eléctricos: %w", err)
	}

	var findings []Finding
	findings = append(findings, DetectPersistentShift(rs, base)...)
	findings = append(findings, DetectTransientDeviation(rs, base)...)
	findings = append(findings, electrical...)
	findings = append(findings, DetectDataQuality(rs)...)
	return Classify(meterID, findings, events), nil
}

// Analyze analiza todos los medidores y devuelve las anomalías ordenadas por prioridad (la mayor primero).
// Los medidores sin anomalías no aparecen. Un error en un medidor aborta el análisis: es preferible fallar
// a entregar un resultado incompleto que parezca completo.
func Analyze(readings map[string][]Reading, events map[string][]Event) ([]Anomaly, error) {
	ids := make([]string, 0, len(readings))
	for id := range readings {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var out []Anomaly
	for _, id := range ids {
		a, err := AnalyzeMeter(id, readings[id], events[id])
		if err != nil {
			return nil, fmt.Errorf("medidor %s: %w", id, err)
		}
		if a != nil {
			out = append(out, *a)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PriorityScore > out[j].PriorityScore })
	return out, nil
}
