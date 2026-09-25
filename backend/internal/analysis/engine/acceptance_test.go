package engine

import "testing"

// Criterios de aceptación de la sección 9 del enunciado (docs/DESIGN.md §1), con los CSV reales.
func TestAcceptanceRealData(t *testing.T) {
	readings, events := realData(t)
	got, err := Analyze(readings, events)
	if err != nil {
		t.Fatal(err)
	}

	want := []struct {
		meter string
		typ   AnomalyType
		sev   Severity
	}{
		{"M-109", AnomalyReal, SeverityHigh},
		{"M-112", AnomalyDataQuality, SeverityHigh},
		{"M-104", AnomalyExplainable, SeverityMedium},
		{"M-106", AnomalyFalsePos, SeverityLow},
	}
	if len(got) != len(want) {
		for _, a := range got {
			t.Logf("%s %s/%s prio=%v", a.MeterID, a.Type, a.Severity, a.PriorityScore)
		}
		t.Fatalf("anomalías = %d, quería %d (los otros 8 medidores no deben tener ninguna)", len(got), len(want))
	}
	for i, w := range want {
		a := got[i]
		if a.MeterID != w.meter || a.Type != w.typ || a.Severity != w.sev {
			t.Errorf("posición %d: %s %s/%s, quería %s %s/%s", i+1, a.MeterID, a.Type, a.Severity, w.meter, w.typ, w.sev)
		}
		if a.Confidence < 0.5 || a.Confidence > 0.99 || a.PriorityScore <= 0 || len(a.Findings) == 0 {
			t.Errorf("%s: anomalía incompleta %+v", a.MeterID, a)
		}
	}
	// M-109 es lo primero que debe ver el operador y nada lo explica.
	if got[0].MeterID != "M-109" {
		t.Errorf("primero = %s, quería M-109", got[0].MeterID)
	}
	if ev := got[0].RelatedEvent; ev == nil || ev.Type != EventUnknown {
		t.Errorf("M-109: evento relacionado = %+v, quería el UNKNOWN", ev)
	}
	// M-104 y M-106 sí tienen un evento que los explica.
	byMeter := map[string]Anomaly{}
	for _, a := range got {
		byMeter[a.MeterID] = a
	}
	if ev := byMeter["M-104"].RelatedEvent; ev == nil || ev.Type != EventOperationalChange {
		t.Errorf("M-104: evento = %+v", ev)
	}
	if ev := byMeter["M-106"].RelatedEvent; ev == nil || ev.Type != EventScheduledOutage {
		t.Errorf("M-106: evento = %+v", ev)
	}
	for _, m := range controlMeters {
		if a, ok := byMeter[m]; ok {
			t.Errorf("%s es un control y no debe tener anomalías: %+v", m, a)
		}
	}
}

func TestAnalyzeIsDeterministic(t *testing.T) {
	readings, events := realData(t)
	a, _ := Analyze(readings, events)
	b, _ := Analyze(readings, events)
	if len(a) != len(b) {
		t.Fatal("longitudes distintas")
	}
	for i := range a {
		if a[i].MeterID != b[i].MeterID || a[i].PriorityScore != b[i].PriorityScore {
			t.Errorf("resultado distinto en la posición %d", i)
		}
	}
}

func TestAnalyzeReportsMeterOnError(t *testing.T) {
	_, err := Analyze(map[string][]Reading{"M-999": series(2, func(_, _ int) float64 { return 1 })}, nil)
	if err == nil {
		t.Fatal("con 2 días de datos debe fallar")
	}
}
