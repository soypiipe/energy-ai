package engine

import "sort"

const (
	// highChangePct: un cambio de consumo de esta magnitud (o mayor) ya es grave por sí solo.
	highChangePct = 50.0
)

// Classify convierte los hallazgos de un medidor en una anomalía clasificada, o nil si no hay hallazgos.
//
// Reglas en orden (docs/DESIGN.md §6); gana la primera que aplica:
//
//  1. DATA_QUALITY (HIGH): hay un hallazgo de calidad de datos. Si el sensor miente, ningún otro
//     hallazgo del medidor es confiable, y hay que arreglar la medición antes de analizar consumo.
//  2. FALSE_POSITIVE (LOW): solo hay una desviación transitoria y un evento la explica (mantenimiento
//     programado): se detectó, pero no requiere acción.
//  3. EXPLAINABLE_ANOMALY (MEDIUM): hay un cambio persistente o eléctrico que un evento explica
//     (cambio operativo). Es real, pero esperado; conviene confirmarlo, no investigarlo de urgencia.
//  4. REAL_ANOMALY: nada lo explica. Severidad HIGH si el consumo cambió ≥50% o el cambio persistente se
//     acompaña de cambios eléctricos; MEDIUM si es un cambio persistente menor, un problema eléctrico
//     eléctrico aislado; LOW si es solo una desviación transitoria (<50%) sin explicación.
//
// Confianza y prioridad se calculan en score.go.
//
// La correlación con eventos se hace desde el inicio del hallazgo principal: persistente, luego
// transitorio, luego eléctrico (y dentro de cada tipo, el de mayor magnitud).
func Classify(meterID string, findings []Finding, events []Event) *Anomaly {
	a, primary := classify(meterID, findings, events)
	if a != nil {
		a.Confidence, a.PriorityScore = score(a, primary)
	}
	return a
}

// classify aplica las reglas y devuelve además el hallazgo principal, que necesita el puntaje.
func classify(meterID string, findings []Finding, events []Event) (*Anomaly, Finding) {
	if len(findings) == 0 {
		return nil, Finding{}
	}
	byKind := map[FindingKind][]Finding{}
	for _, f := range findings {
		byKind[f.Kind] = append(byKind[f.Kind], f)
	}
	a := &Anomaly{MeterID: meterID, Findings: sortedFindings(findings)}

	if dq := byKind[FindingDataQuality]; len(dq) > 0 {
		a.Type, a.Severity, a.DetectedAt = AnomalyDataQuality, SeverityHigh, dq[0].Start
		a.RelatedEvent, _ = CorrelateEvent(dq[0], events)
		return a, dq[0]
	}

	primary := primaryFinding(byKind)
	a.DetectedAt = primary.Start
	related, explained := CorrelateEvent(primary, events)
	a.RelatedEvent = related

	persistent, electrical := len(byKind[FindingPersistentShift]) > 0, len(byKind[FindingElectricalChange]) > 0
	switch {
	case explained && primary.Kind == FindingTransientDeviation:
		a.Type, a.Severity = AnomalyFalsePos, SeverityLow
	case explained:
		a.Type, a.Severity = AnomalyExplainable, SeverityMedium
	default:
		a.Type = AnomalyReal
		change := abs(consumptionChange(a.Findings))
		switch {
		case change >= highChangePct, persistent && electrical:
			a.Severity = SeverityHigh
		case primary.Kind != FindingTransientDeviation:
			a.Severity = SeverityMedium
		default:
			a.Severity = SeverityLow
		}
	}
	return a, primary
}

// primaryFinding elige el hallazgo que define la anomalía: persistente > transitorio > eléctrico.
func primaryFinding(byKind map[FindingKind][]Finding) Finding {
	for _, k := range []FindingKind{FindingPersistentShift, FindingTransientDeviation, FindingElectricalChange} {
		if fs := byKind[k]; len(fs) > 0 {
			best := fs[0]
			for _, f := range fs[1:] {
				if magnitude(f) > magnitude(best) {
					best = f
				}
			}
			return best
		}
	}
	return Finding{}
}

// magnitude es el mayor |cambio %| de la evidencia de un hallazgo.
func magnitude(f Finding) float64 {
	var m float64
	for _, e := range f.Evidence {
		m = max(m, abs(e.ChangePct))
	}
	return m
}

// consumptionChange es el cambio % de consumo del hallazgo de consumo más fuerte (con signo); 0 si no hay.
func consumptionChange(fs []Finding) float64 {
	var best float64
	for _, f := range fs {
		for _, e := range f.Evidence {
			if e.Metric == string(MetricConsumption) && abs(e.ChangePct) > abs(best) {
				best = e.ChangePct
			}
		}
	}
	return best
}

func sortedFindings(fs []Finding) []Finding {
	out := append([]Finding(nil), fs...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}
