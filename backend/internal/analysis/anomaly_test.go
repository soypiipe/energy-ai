package analysis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// analyzed devuelve un servidor con los CSV sembrados y un análisis ya completado.
func analyzed(t *testing.T) (*Repository, http.Handler, string) {
	t.Helper()
	ctx := context.Background()
	repo := seeded(t)
	mux := http.NewServeMux()
	NewHandler(repo).Register(mux)

	run, _ := repo.Enqueue(ctx)
	repo.Claim(ctx)
	summary, err := NewAnalyzer(repo).Process(ctx, run, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Complete(ctx, run.ID, summary); err != nil {
		t.Fatal(err)
	}
	return repo, mux, run.ID
}

func TestListAnomaliesOrderedAndFiltered(t *testing.T) {
	_, h, runID := analyzed(t)

	code, body, _ := do(t, h, "GET", "/anomalies")
	if code != 200 || body["analysis_id"] != runID {
		t.Fatalf("GET /anomalies = %d %v", code, body)
	}
	data := body["data"].([]any)
	if len(data) != 4 {
		t.Fatalf("anomalías = %d, quería 4", len(data))
	}
	first := data[0].(map[string]any)
	if first["meter_id"] != "M-109" || first["type"] != "REAL_ANOMALY" || first["meter_name"] == "" || first["status"] != "OPEN" {
		t.Errorf("primera = %v", first)
	}
	if _, ok := first["evidence"].(map[string]any); !ok {
		t.Errorf("evidence debe ser un objeto JSON: %v", first["evidence"])
	}

	for query, want := range map[string]int{
		"?severity=HIGH":                   2,
		"?type=FALSE_POSITIVE":             1,
		"?meter_id=M-104":                  1,
		"?meter_id=M-101":                  0,
		"?status=RESOLVED":                 0,
		"?severity=HIGH&type=DATA_QUALITY": 1,
	} {
		_, b, _ := do(t, h, "GET", "/anomalies"+query)
		if got := len(b["data"].([]any)); got != want {
			t.Errorf("%s → %d anomalías, quería %d", query, got, want)
		}
	}
}

func TestListAnomaliesEmptyBeforeAnyAnalysis(t *testing.T) {
	_, h := newServer(t)
	code, body, _ := do(t, h, "GET", "/anomalies")
	if code != 200 || body["analysis_id"] != nil || len(body["data"].([]any)) != 0 {
		t.Errorf("GET = %d %v", code, body)
	}
}

func TestListAnomaliesRejectsBadFilters(t *testing.T) {
	_, h := newServer(t)
	for _, q := range []string{"?meter_id=" + url.QueryEscape("x'; DROP TABLE anomalies;--"), "?type=NOPE", "?severity=CRITICAL", "?status=DONE"} {
		if code, _, _ := do(t, h, "GET", "/anomalies"+q); code != http.StatusBadRequest {
			t.Errorf("%s → %d, quería 400", q, code)
		}
	}
}

func TestGetAndPatchAnomaly(t *testing.T) {
	_, h, _ := analyzed(t)
	_, list, _ := do(t, h, "GET", "/anomalies")
	id := list["data"].([]any)[0].(map[string]any)["id"].(string)

	if code, body, _ := do(t, h, "GET", "/anomalies/"+id); code != 200 || body["id"] != id {
		t.Fatalf("GET /anomalies/{id} = %d %v", code, body)
	}

	patch := func(path, payload string) (int, map[string]any) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("PATCH", path, strings.NewReader(payload)))
		var b map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &b)
		return rec.Code, b
	}
	if code, b := patch("/anomalies/"+id, `{"status":"ACKNOWLEDGED"}`); code != 200 || b["status"] != "ACKNOWLEDGED" {
		t.Errorf("PATCH válido = %d %v", code, b)
	}
	_, after, _ := do(t, h, "GET", "/anomalies?status=ACKNOWLEDGED")
	if len(after["data"].([]any)) != 1 {
		t.Error("el cambio de estado no persistió")
	}

	for name, payload := range map[string]string{
		"estado inválido": `{"status":"DONE"}`,
		"campo extra":     `{"status":"OPEN","severity":"LOW"}`,
		"no es JSON":      `nope`,
		"vacío":           `{}`,
	} {
		if code, _ := patch("/anomalies/"+id, payload); code != http.StatusBadRequest {
			t.Errorf("%s → %d, quería 400", name, code)
		}
	}
	if code, _ := patch("/anomalies/00000000-0000-0000-0000-000000000000", `{"status":"OPEN"}`); code != http.StatusNotFound {
		t.Errorf("inexistente → %d", code)
	}
	if code, _ := patch("/anomalies/xyz", `{"status":"OPEN"}`); code != http.StatusBadRequest {
		t.Errorf("id inválido → %d", code)
	}
	if code, _, _ := do(t, h, "GET", "/anomalies/00000000-0000-0000-0000-000000000000"); code != http.StatusNotFound {
		t.Errorf("GET inexistente → %d", code)
	}
}
