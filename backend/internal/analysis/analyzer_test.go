package analysis

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/soypiipe/energy-ai/backend/internal/pgtest"
	"github.com/soypiipe/energy-ai/backend/internal/seed"
)

// seeded devuelve un repositorio con los CSV reales cargados.
func seeded(t *testing.T) *Repository {
	t.Helper()
	pool := pgtest.New(t)
	if _, err := seed.Run(context.Background(), pool, "../../../data"); err != nil {
		t.Fatal(err)
	}
	return NewRepository(pool)
}

func TestAnalyzerEndToEnd(t *testing.T) {
	ctx := context.Background()
	repo := seeded(t)

	w := NewWorker(repo, NewAnalyzer(repo).Process)
	stop := runWorker(t, w)
	defer stop()

	run, _ := repo.Enqueue(ctx)
	got := waitStatus(t, repo, run.ID, StatusCompleted)

	var sum Summary
	if err := json.Unmarshal(got.Summary, &sum); err != nil {
		t.Fatal(err)
	}
	if sum.MetersAnalyzed != 12 || sum.Anomalies != 4 || sum.ByType["REAL_ANOMALY"] != 1 || sum.ByType["DATA_QUALITY"] != 1 {
		t.Errorf("resumen = %+v", sum)
	}

	rows, err := repo.pool.Query(ctx, `
		SELECT meter_id, type, severity, priority_score, evidence
		FROM anomalies WHERE analysis_id = $1 ORDER BY priority_score DESC`, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	want := []struct{ meter, typ, sev string }{
		{"M-109", "REAL_ANOMALY", "HIGH"},
		{"M-112", "DATA_QUALITY", "HIGH"},
		{"M-104", "EXPLAINABLE_ANOMALY", "MEDIUM"},
		{"M-106", "FALSE_POSITIVE", "LOW"},
	}
	i := 0
	for rows.Next() {
		var meter, typ, sev string
		var prio float64
		var evidence []byte
		if err := rows.Scan(&meter, &typ, &sev, &prio, &evidence); err != nil {
			t.Fatal(err)
		}
		if i >= len(want) || meter != want[i].meter || typ != want[i].typ || sev != want[i].sev {
			t.Errorf("fila %d: %s %s/%s", i, meter, typ, sev)
		}
		if meter == "M-109" {
			var doc struct {
				Findings []struct {
					Kind  string  `json:"kind"`
					Start string  `json:"start"`
					End   *string `json:"end"`
				} `json:"findings"`
				RelatedEvent *struct{ Type string } `json:"related_event"`
			}
			if err := json.Unmarshal(evidence, &doc); err != nil {
				t.Fatal(err)
			}
			if len(doc.Findings) == 0 || doc.RelatedEvent == nil || doc.RelatedEvent.Type != "UNKNOWN" {
				t.Errorf("evidencia de M-109 incompleta: %s", evidence)
			}
			if _, err := time.Parse(localLayout, doc.Findings[0].Start); err != nil {
				t.Errorf("start sin formato local: %q", doc.Findings[0].Start)
			}
		}
		i++
	}
	if i != len(want) {
		t.Errorf("filas = %d, quería %d", i, len(want))
	}
}

func TestSaveAnomaliesIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := seeded(t)
	run, _ := repo.Enqueue(ctx)
	readings, events, err := repo.LoadDataset(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(readings["M-109"]) != 336 || len(events["M-109"]) != 1 {
		t.Fatalf("dataset: %d lecturas, %d eventos de M-109", len(readings["M-109"]), len(events["M-109"]))
	}
	an := NewAnalyzer(repo)
	repo.Claim(ctx)
	for i := 0; i < 2; i++ { // procesar dos veces la misma ejecución
		if _, err := an.Process(ctx, run, func(string) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := repo.pool.QueryRow(ctx, `SELECT count(*) FROM anomalies WHERE analysis_id = $1`, run.ID).Scan(&n); err != nil || n != 4 {
		t.Errorf("anomalías = %d (err %v), quería 4 sin duplicados", n, err)
	}
}
