package analysis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newServer(t *testing.T) (*Repository, http.Handler) {
	t.Helper()
	repo := newRepo(t)
	mux := http.NewServeMux()
	NewHandler(repo).Register(mux)
	return repo, mux
}

func do(t *testing.T, h http.Handler, method, path string) (int, map[string]any, http.Header) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body, rec.Header()
}

func TestAnalyzeReturns202AndStatusEndpointFollows(t *testing.T) {
	repo, h := newServer(t)

	code, body, hdr := do(t, h, "POST", "/ai/analyze")
	if code != http.StatusAccepted || body["status"] != "PENDING" {
		t.Fatalf("POST = %d %v", code, body)
	}
	id, _ := body["id"].(string)
	if hdr.Get("Location") != "/ai/analysis/"+id {
		t.Errorf("Location = %q", hdr.Get("Location"))
	}

	// el worker toma la ejecución y avanza de paso: el GET lo refleja
	repo.Claim(context.Background())
	repo.SetStep(context.Background(), id, "detectando")
	code, body, _ = do(t, h, "GET", "/ai/analysis/"+id)
	if code != 200 || body["status"] != "RUNNING" || body["current_step"] != "detectando" {
		t.Errorf("GET = %d %v", code, body)
	}
}

func TestAnalyzeReusesActiveRun(t *testing.T) {
	_, h := newServer(t)
	_, first, _ := do(t, h, "POST", "/ai/analyze")
	code, second, _ := do(t, h, "POST", "/ai/analyze")
	if code != http.StatusAccepted || first["id"] != second["id"] {
		t.Errorf("segundo POST debía devolver la misma ejecución: %v vs %v", first["id"], second["id"])
	}
}

func TestAnalyzeCreatesNewRunAfterCompletion(t *testing.T) {
	repo, h := newServer(t)
	_, first, _ := do(t, h, "POST", "/ai/analyze")
	repo.Claim(context.Background())
	repo.Complete(context.Background(), first["id"].(string), map[string]int{"anomalies": 4})

	_, second, _ := do(t, h, "POST", "/ai/analyze")
	if first["id"] == second["id"] {
		t.Error("tras completarse, un POST nuevo debe crear otra ejecución")
	}
}

func TestGetRunHidesInternalError(t *testing.T) {
	repo, h := newServer(t)
	run, _ := repo.Enqueue(context.Background())
	repo.Claim(context.Background())
	repo.Fail(context.Background(), run.ID, `pq: relation "readings" does not exist at /app/secret.go:12`)

	code, body, _ := do(t, h, "GET", "/ai/analysis/"+run.ID)
	if code != 200 || body["status"] != "FAILED" {
		t.Fatalf("GET = %d %v", code, body)
	}
	msg, _ := body["error"].(string)
	if msg == "" || strings.Contains(msg, "relation") || strings.Contains(msg, "secret.go") {
		t.Errorf("el error interno se filtró al cliente: %q", msg)
	}
}

func TestGetRunValidation(t *testing.T) {
	_, h := newServer(t)
	if code, _, _ := do(t, h, "GET", "/ai/analysis/no-es-uuid"); code != http.StatusBadRequest {
		t.Errorf("id inválido: %d", code)
	}
	if code, _, _ := do(t, h, "GET", "/ai/analysis/00000000-0000-0000-0000-000000000000"); code != http.StatusNotFound {
		t.Errorf("inexistente: %d", code)
	}
}
