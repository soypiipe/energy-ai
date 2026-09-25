package explain

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeLLM devuelve un servidor que responde con `content` como mensaje del asistente.
func fakeLLM(t *testing.T, status int, content string, seen *http.Request, seenBody *[]byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			*seen = *r.Clone(context.Background())
			b, _ := io.ReadAll(r.Body)
			*seenBody = b
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": content}}},
		})
	}))
}

const goodJSON = `{"reason":"El consumo subió 100%.","recommended_action":"Inspeccionar M-109."}`

func TestLLMSuccessAndRequestShape(t *testing.T) {
	var req http.Request
	var body []byte
	srv := fakeLLM(t, 200, goodJSON, &req, &body)
	defer srv.Close()

	l := NewLLM(srv.URL+"/", "modelo-x", "sk-secreta")
	got, err := l.Explain(context.Background(), m109())
	if err != nil {
		t.Fatal(err)
	}
	if got.Reason != "El consumo subió 100%." || got.RecommendedAction != "Inspeccionar M-109." {
		t.Errorf("explicación = %+v", got)
	}
	if req.URL.Path != "/chat/completions" || req.Method != "POST" || req.Header.Get("Authorization") != "Bearer sk-secreta" {
		t.Errorf("petición: %s %s auth=%q", req.Method, req.URL.Path, req.Header.Get("Authorization"))
	}

	var sent chatRequest
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatal(err)
	}
	if sent.Model != "modelo-x" || len(sent.Messages) != 2 || sent.Messages[0].Role != "system" {
		t.Errorf("cuerpo: %+v", sent)
	}
	user := sent.Messages[1].Content
	for _, want := range []string{`"meter_id":"M-109"`, `"type":"REAL_ANOMALY"`, `"change_pct":100`, `"explains_the_change":false`} {
		if !strings.Contains(user, want) {
			t.Errorf("el mensaje del usuario no contiene %s:\n%s", want, user)
		}
	}
	if strings.Contains(string(body), "sk-secreta") {
		t.Error("la clave no debe viajar en el cuerpo")
	}
}

func TestLLMAcceptsFencedJSON(t *testing.T) {
	srv := fakeLLM(t, 200, "```json\n"+goodJSON+"\n```", nil, nil)
	defer srv.Close()
	if _, err := NewLLM(srv.URL, "m", "k").Explain(context.Background(), m109()); err != nil {
		t.Errorf("JSON entre ``` debía aceptarse: %v", err)
	}
}

func TestLLMRejectsBadOutput(t *testing.T) {
	cases := map[string]struct {
		status  int
		content string
	}{
		"no es JSON":      {200, "Claro, aquí tienes la explicación..."},
		"reason vacío":    {200, `{"reason":"","recommended_action":"x"}`},
		"falta la acción": {200, `{"reason":"x"}`},
		"demasiado largo": {200, `{"reason":"` + strings.Repeat("a", 700) + `","recommended_action":"x"}`},
		"HTTP 500":        {500, goodJSON},
		"HTTP 401":        {401, goodJSON},
	}
	for name, c := range cases {
		srv := fakeLLM(t, c.status, c.content, nil, nil)
		if _, err := NewLLM(srv.URL, "m", "k").Explain(context.Background(), m109()); err == nil {
			t.Errorf("%s: debía fallar", name)
		}
		srv.Close()
	}
}

func TestLLMEmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()
	if _, err := NewLLM(srv.URL, "m", "k").Explain(context.Background(), m109()); err == nil {
		t.Error("sin choices debía fallar")
	}
}

func TestLLMTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()
	l := NewLLM(srv.URL, "m", "k")
	l.timeout = 50 * time.Millisecond
	start := time.Now()
	if _, err := l.Explain(context.Background(), m109()); err == nil {
		t.Fatal("debía fallar por timeout")
	}
	if time.Since(start) > time.Second {
		t.Errorf("tardó %v: el timeout no se respetó", time.Since(start))
	}
}

func TestLLMErrorNeverLeaksKey(t *testing.T) {
	// URL inválida → el error de net/http podría incluir la URL; la clave no debe aparecer nunca.
	l := NewLLM("http://127.0.0.1:1", "m", "sk-secreta")
	_, err := l.Explain(context.Background(), m109())
	if err == nil || strings.Contains(err.Error(), "sk-secreta") {
		t.Errorf("err = %v", err)
	}
}
