package engine

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

func fnd(kind FindingKind, change float64, metric Metric) Finding {
	return Finding{Kind: kind, Start: t0, Evidence: []Evidence{{Metric: string(metric), Baseline: 100, Observed: 100 + change, ChangePct: change}}}
}

func TestClassifyRules(t *testing.T) {
	outage := []Event{{Timestamp: t0, Type: EventScheduledOutage}}
	opchange := []Event{{Timestamp: t0.Add(time.Hour), Type: EventOperationalChange}}
	unknown := []Event{{Timestamp: t0, Type: EventUnknown}}

	tests := []struct {
		name     string
		findings []Finding
		events   []Event
		wantType AnomalyType
		wantSev  Severity
	}{
		{"calidad de datos gana a todo", []Finding{fnd(FindingPersistentShift, 100, MetricConsumption), fnd(FindingDataQuality, 0, "kwh_vi_ratio")}, opchange, AnomalyDataQuality, SeverityHigh},
		{"transitoria con apagado programado", []Finding{fnd(FindingTransientDeviation, -80, MetricConsumption)}, outage, AnomalyFalsePos, SeverityLow},
		{"persistente con cambio operativo", []Finding{fnd(FindingPersistentShift, 47, MetricConsumption), fnd(FindingElectricalChange, 47, MetricCurrent)}, opchange, AnomalyExplainable, SeverityMedium},
		{"persistente +100% con UNKNOWN", []Finding{fnd(FindingPersistentShift, 100, MetricConsumption), fnd(FindingElectricalChange, 100, MetricCurrent)}, unknown, AnomalyReal, SeverityHigh},
		{"persistente +30% sin evento ni eléctrico", []Finding{fnd(FindingPersistentShift, 30, MetricConsumption)}, nil, AnomalyReal, SeverityMedium},
		{"persistente +30% con eléctrico", []Finding{fnd(FindingPersistentShift, 30, MetricConsumption), fnd(FindingElectricalChange, -10, MetricPowerFactor)}, nil, AnomalyReal, SeverityHigh},
		{"solo eléctrico sin evento", []Finding{fnd(FindingElectricalChange, -12, MetricPowerFactor)}, nil, AnomalyReal, SeverityMedium},
		{"transitoria sin explicación", []Finding{fnd(FindingTransientDeviation, -40, MetricConsumption)}, unknown, AnomalyReal, SeverityLow},
		{"transitoria grande sin explicación", []Finding{fnd(FindingTransientDeviation, -80, MetricConsumption)}, nil, AnomalyReal, SeverityHigh},
	}
	for _, tc := range tests {
		a := Classify("M-X", tc.findings, tc.events)
		if a == nil || a.Type != tc.wantType || a.Severity != tc.wantSev {
			t.Errorf("%s: got %+v, quería %s/%s", tc.name, a, tc.wantType, tc.wantSev)
		}
	}
}

func TestClassifyNoFindings(t *testing.T) {
	if a := Classify("M-X", nil, nil); a != nil {
		t.Errorf("sin hallazgos no hay anomalía: %+v", a)
	}
}

func TestClassifyFillsContext(t *testing.T) {
	ev := []Event{{Timestamp: t0, Type: EventUnknown, Description: "No operational event reported"}}
	a := Classify("M-109", []Finding{fnd(FindingPersistentShift, 100, MetricConsumption)}, ev)
	if a.MeterID != "M-109" || !a.DetectedAt.Equal(t0) || a.RelatedEvent == nil || a.RelatedEvent.Type != EventUnknown || len(a.Findings) != 1 {
		t.Errorf("anomalía = %+v", a)
	}
}
