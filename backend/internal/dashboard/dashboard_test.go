package dashboard

import (
	"testing"
	"time"

	"github.com/soypiipe/energy-ai/backend/internal/analysis"
	"github.com/soypiipe/energy-ai/backend/internal/meter"
)

func TestBuildEmpty(t *testing.T) {
	s := Build(nil, nil, nil)
	if s.Meters.Total != 0 || s.TopPriority != nil || s.LastAnalysis != nil || s.Anomalies.Total != 0 {
		t.Errorf("vacío = %+v", s)
	}
	if s.Anomalies.BySeverity == nil || s.Anomalies.ByType == nil {
		t.Error("los mapas deben serializarse como {} y no como null")
	}
}

func TestBuild(t *testing.T) {
	meters := []meter.Summary{
		{MeterID: "M-101", Status: meter.StatusOK, TotalKWh: 100, CurrentKWh: 10, BaselineKWh: 10},
		{MeterID: "M-104", Status: meter.StatusAlert, TotalKWh: 200, CurrentKWh: 15, BaselineKWh: 10},
		{MeterID: "M-109", Status: meter.StatusCritical, TotalKWh: 300, CurrentKWh: 20, BaselineKWh: 10},
	}
	anomalies := []analysis.Anomaly{
		{ID: "a1", MeterID: "M-109", Type: "REAL_ANOMALY", Severity: "HIGH", Status: analysis.AnomalyOpen},
		{ID: "a2", MeterID: "M-104", Type: "EXPLAINABLE_ANOMALY", Severity: "MEDIUM", Status: analysis.AnomalyResolved},
	}
	fin := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	s := Build(meters, anomalies, &analysis.Run{ID: "run1", FinishedAt: &fin})

	if s.Meters != (MeterCounts{Total: 3, OK: 1, Alert: 1, Critical: 1}) {
		t.Errorf("medidores = %+v", s.Meters)
	}
	if c := s.Consumption; c.TotalKWh != 600 || c.CurrentKWh != 45 || c.BaselineKWh != 30 || c.VariationPct != 50 {
		t.Errorf("consumo = %+v", c)
	}
	if a := s.Anomalies; a.Total != 2 || a.Open != 1 || a.BySeverity["HIGH"] != 1 || a.ByType["REAL_ANOMALY"] != 1 {
		t.Errorf("anomalías = %+v", a)
	}
	if s.TopPriority == nil || s.TopPriority.MeterID != "M-109" || s.TopPriority.AnomalyID != "a1" {
		t.Errorf("top = %+v", s.TopPriority)
	}
	if s.LastAnalysis == nil || s.LastAnalysis.ID != "run1" {
		t.Errorf("último análisis = %+v", s.LastAnalysis)
	}
}
