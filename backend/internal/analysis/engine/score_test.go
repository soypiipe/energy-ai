package engine

import "testing"

func TestScoreFormulas(t *testing.T) {
	// REAL, +100% consumo, corriente ×2 (2 tipos → corroboración 0.5), activo, HIGH:
	// confianza = 0.5 + 0.2·1 + 0.15·0.5 + 0.15·1 = 0.925 → 0.93 ; prioridad = 3·1·0.93 = 2.79
	a := Classify("M-X", []Finding{fnd(FindingPersistentShift, 100, MetricConsumption), fnd(FindingElectricalChange, 100, MetricCurrent)}, nil)
	if a.Confidence != 0.93 || a.PriorityScore != 2.79 {
		t.Errorf("REAL: conf=%v prio=%v, quería 0.93 / 2.79", a.Confidence, a.PriorityScore)
	}

	// FALSE_POSITIVE, -80%, evento exacto, un solo tipo, ya terminó:
	// confianza = 0.5 + 0.2·0.8 + 0 + 0.15·1 = 0.81 ; prioridad = 1·(0.5+0.4)·0.81 = 0.73
	f := fnd(FindingTransientDeviation, -80, MetricConsumption)
	f.End = f.Start.Add(11 * 3600e9)
	a = Classify("M-X", []Finding{f}, []Event{{Timestamp: t0, Type: EventScheduledOutage}})
	if a.Confidence != 0.81 || a.PriorityScore != 0.73 {
		t.Errorf("FALSE_POSITIVE: conf=%v prio=%v, quería 0.81 / 0.73", a.Confidence, a.PriorityScore)
	}
}

func TestScoreBounds(t *testing.T) {
	cases := [][]Finding{
		{fnd(FindingPersistentShift, 500, MetricConsumption), fnd(FindingElectricalChange, 300, MetricCurrent), fnd(FindingDataQuality, 0, "kwh_vi_ratio")},
		{fnd(FindingElectricalChange, -9, MetricPowerFactor)},
	}
	for _, fs := range cases {
		a := Classify("M-X", fs, nil)
		if a.Confidence < 0.5 || a.Confidence > 0.99 || a.PriorityScore <= 0 {
			t.Errorf("fuera de rango: %+v", a)
		}
	}
}
