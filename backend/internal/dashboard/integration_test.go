package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/soypiipe/energy-ai/backend/internal/analysis"
	"github.com/soypiipe/energy-ai/backend/internal/meter"
	"github.com/soypiipe/energy-ai/backend/internal/pgtest"
	"github.com/soypiipe/energy-ai/backend/internal/seed"
)

func TestSummaryEndpointWithRealData(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	if _, err := seed.Run(ctx, pool, "../../../data"); err != nil {
		t.Fatal(err)
	}
	arepo := analysis.NewRepository(pool)
	mux := http.NewServeMux()
	NewHandler(meter.NewRepository(pool), arepo).Register(mux)

	get := func() Summary {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/dashboard/summary", nil))
		if rec.Code != 200 {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var s Summary
		if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	// antes de analizar: 12 medidores, todos OK, sin anomalías ni análisis
	before := get()
	if before.Meters.Total != 12 || before.Meters.OK != 12 || before.Anomalies.Total != 0 || before.LastAnalysis != nil || before.TopPriority != nil {
		t.Errorf("antes = %+v", before)
	}

	// se analiza: M-109 crítico, M-104 y M-112 en alerta, M-106 (falso positivo) sigue OK
	run, _ := arepo.Enqueue(ctx)
	arepo.Claim(ctx)
	summary, err := analysis.NewAnalyzer(arepo).Process(ctx, run, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := arepo.Complete(ctx, run.ID, summary); err != nil {
		t.Fatal(err)
	}

	after := get()
	if after.Meters != (MeterCounts{Total: 12, OK: 9, Alert: 2, Critical: 1}) {
		t.Errorf("medidores = %+v", after.Meters)
	}
	if after.Anomalies.Total != 4 || after.Anomalies.Open != 4 || after.Anomalies.BySeverity["HIGH"] != 2 {
		t.Errorf("anomalías = %+v", after.Anomalies)
	}
	if after.TopPriority == nil || after.TopPriority.MeterID != "M-109" {
		t.Errorf("top = %+v", after.TopPriority)
	}
	if after.LastAnalysis == nil || after.LastAnalysis.ID != run.ID || after.Consumption.TotalKWh <= 0 {
		t.Errorf("resto = %+v", after)
	}
}
