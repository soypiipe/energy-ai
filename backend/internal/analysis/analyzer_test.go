package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/soypiipe/energy-ai/backend/internal/analysis/engine"
	"github.com/soypiipe/energy-ai/backend/internal/explain"
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

	w := NewWorker(repo, NewAnalyzer(repo, explain.NewTemplate()).Process)
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
	an := NewAnalyzer(repo, explain.NewTemplate())
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

// runAnalysis corre un análisis completo con el explainer dado y devuelve resumen y textos guardados.
func runAnalysis(t *testing.T, repo *Repository, ex explain.Explainer) (Summary, map[string][2]string, map[string]string) {
	t.Helper()
	ctx := context.Background()
	run, _ := repo.Enqueue(ctx)
	if c, err := repo.Claim(ctx); err != nil || c == nil {
		t.Fatalf("claim: %v %v", c, err)
	}
	sum, err := NewAnalyzer(repo, ex).Process(ctx, run, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Complete(ctx, run.ID, sum); err != nil {
		t.Fatal(err)
	}
	_, list, err := repo.ListAnomalies(ctx, AnomalyFilter{})
	if err != nil {
		t.Fatal(err)
	}
	texts := map[string][2]string{}
	sources := map[string]string{}
	for _, a := range list {
		texts[a.MeterID] = [2]string{a.Reason, a.RecommendedAction}
		var doc struct {
			Source string `json:"explanation_source"`
		}
		_ = json.Unmarshal(a.Evidence, &doc)
		sources[a.MeterID] = doc.Source
	}
	return sum.(Summary), texts, sources
}

func fakeLLMServer(status int, content string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": content}}}})
	}))
}

// El análisis debe dar 4 anomalías con texto tanto con LLM sano, con LLM caído como sin LLM.
func TestAnalysisWorksWithAndWithoutLLM(t *testing.T) {
	good := fakeLLMServer(200, `{"reason":"Texto del LLM.","recommended_action":"Acción del LLM."}`)
	defer good.Close()
	broken := fakeLLMServer(500, "")
	defer broken.Close()

	cases := []struct {
		name       string
		explainer  explain.Explainer
		wantSource string
	}{
		{"sin LLM_API_KEY (solo plantilla)", explain.NewTemplate(), explain.SourceTemplate},
		{"LLM sano", explain.NewFallback(explain.NewLLM(good.URL, "m", "k"), explain.NewTemplate()), explain.SourceLLM},
		{"LLM caído (HTTP 500)", explain.NewFallback(explain.NewLLM(broken.URL, "m", "k"), explain.NewTemplate()), explain.SourceTemplate},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sum, texts, sources := runAnalysis(t, seeded(t), c.explainer)
			if sum.Anomalies != 4 || sum.ExplainedBy[c.wantSource] != 4 {
				t.Errorf("resumen = %+v", sum)
			}
			if len(texts) != 4 {
				t.Fatalf("textos = %d", len(texts))
			}
			for meter, tx := range texts {
				if tx[0] == "" || tx[1] == "" {
					t.Errorf("%s sin texto: %v", meter, tx)
				}
				if sources[meter] != c.wantSource {
					t.Errorf("%s fuente = %q, quería %q", meter, sources[meter], c.wantSource)
				}
			}
		})
	}
}

func TestAnalysisFailsIfExplainerFails(t *testing.T) {
	ctx := context.Background()
	repo := seeded(t)
	run, _ := repo.Enqueue(ctx)
	repo.Claim(ctx)
	failing := explainerFunc(func(context.Context, engine.Anomaly) (explain.Explanation, error) {
		return explain.Explanation{}, errors.New("sin texto")
	})
	if _, err := NewAnalyzer(repo, failing).Process(ctx, run, func(string) error { return nil }); err == nil {
		t.Error("si no hay explicación no se deben guardar anomalías sin texto")
	}
}

type explainerFunc func(context.Context, engine.Anomaly) (explain.Explanation, error)

func (f explainerFunc) Explain(ctx context.Context, a engine.Anomaly) (explain.Explanation, error) {
	return f(ctx, a)
}
