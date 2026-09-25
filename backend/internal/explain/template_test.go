package explain

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/soypiipe/energy-ai/backend/internal/analysis/engine"
)

var at = time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)

func ev(metric string, base, obs, change float64) engine.Evidence {
	return engine.Evidence{Metric: metric, Baseline: base, Observed: obs, ChangePct: change}
}

func m109() engine.Anomaly {
	return engine.Anomaly{
		MeterID: "M-109", DetectedAt: at, Type: engine.AnomalyReal, Severity: engine.SeverityHigh, Confidence: 0.93, PriorityScore: 2.79,
		Findings: []engine.Finding{
			{Kind: engine.FindingPersistentShift, Start: at, Evidence: []engine.Evidence{ev("consumption_kwh", 43.9, 87.8, 100)}},
			{Kind: engine.FindingElectricalChange, Start: at, Evidence: []engine.Evidence{ev("current_a", 200, 420, 110), ev("power_factor", 0.94, 0.74, -21.3)}},
		},
		RelatedEvent: &engine.Event{Timestamp: at, Type: engine.EventUnknown, Description: "No operational event reported"},
	}
}

func TestTemplateReal(t *testing.T) {
	got, err := NewTemplate().Explain(context.Background(), m109())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"subió 100%", "43,9", "87,8", "12 sep 14:00", "sigue así", "la corriente sube 110%", "el factor de potencia baja 21,3%", "No operational event reported", "no explica"} {
		if !strings.Contains(got.Reason, want) {
			t.Errorf("reason no contiene %q:\n%s", want, got.Reason)
		}
	}
	if !strings.Contains(got.RecommendedAction, "M-109") || !strings.Contains(got.RecommendedAction, "factor de potencia") {
		t.Errorf("acción: %s", got.RecommendedAction)
	}
}

func TestTemplateIsDeterministic(t *testing.T) {
	a, _ := NewTemplate().Explain(context.Background(), m109())
	b, _ := NewTemplate().Explain(context.Background(), m109())
	if a != b {
		t.Error("la plantilla debe ser determinista")
	}
}

func TestTemplateExplainable(t *testing.T) {
	s := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	a := engine.Anomaly{
		MeterID: "M-104", Type: engine.AnomalyExplainable, Severity: engine.SeverityMedium,
		Findings:     []engine.Finding{{Kind: engine.FindingPersistentShift, Start: s, Evidence: []engine.Evidence{ev("consumption_kwh", 48.6, 71.4, 46.9)}}},
		RelatedEvent: &engine.Event{Timestamp: s, Type: engine.EventOperationalChange, Description: "New production line activated"},
	}
	got, err := NewTemplate().Explain(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Reason, "cambio operativo") || !strings.Contains(got.Reason, "New production line activated") || !strings.Contains(got.Reason, "46,9%") {
		t.Errorf("reason: %s", got.Reason)
	}
	if !strings.Contains(got.RecommendedAction, "operaciones") {
		t.Errorf("acción: %s", got.RecommendedAction)
	}
}

func TestTemplateFalsePositive(t *testing.T) {
	s := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	a := engine.Anomaly{
		MeterID: "M-106", Type: engine.AnomalyFalsePos, Severity: engine.SeverityLow,
		Findings:     []engine.Finding{{Kind: engine.FindingTransientDeviation, Start: s, End: s.Add(11 * time.Hour), Evidence: []engine.Evidence{ev("consumption_kwh", 56, 11.2, -80)}}},
		RelatedEvent: &engine.Event{Timestamp: s, Type: engine.EventScheduledOutage, Description: "Scheduled maintenance outage for 12 hours"},
	}
	got, err := NewTemplate().Explain(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Reason, "cayó") || !strings.Contains(got.Reason, "12 horas") || !strings.Contains(got.Reason, "apagado programado") {
		t.Errorf("reason: %s", got.Reason)
	}
	if !strings.Contains(got.RecommendedAction, "No requiere acción") {
		t.Errorf("acción: %s", got.RecommendedAction)
	}
}

func TestTemplateDataQuality(t *testing.T) {
	s := time.Date(2026, 9, 13, 3, 0, 0, 0, time.UTC)
	a := engine.Anomaly{
		MeterID: "M-112", Type: engine.AnomalyDataQuality, Severity: engine.SeverityHigh,
		Findings: []engine.Finding{{Kind: engine.FindingDataQuality, Start: s, Evidence: []engine.Evidence{ev("kwh_vi_ratio", 1.06, 2.35, 121.7), {Metric: "flagged_readings", Observed: 13}}}},
	}
	got, err := NewTemplate().Explain(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Reason, "13 lecturas") || !strings.Contains(got.Reason, "1,06") || !strings.Contains(got.Reason, "no son confiables") {
		t.Errorf("reason: %s", got.Reason)
	}
	if !strings.Contains(got.RecommendedAction, "sensores") {
		t.Errorf("acción: %s", got.RecommendedAction)
	}
}

func TestValidate(t *testing.T) {
	ok := Explanation{Reason: "a", RecommendedAction: "b"}
	if err := ok.Validate(); err != nil {
		t.Error(err)
	}
	bad := []Explanation{
		{Reason: "", RecommendedAction: "b"},
		{Reason: "a", RecommendedAction: "  "},
		{Reason: strings.Repeat("x", maxReasonLen+1), RecommendedAction: "b"},
		{Reason: "a", RecommendedAction: strings.Repeat("ñ", maxActionLen+1)},
	}
	for i, e := range bad {
		if e.Validate() == nil {
			t.Errorf("caso %d debía ser inválido", i)
		}
	}
}
