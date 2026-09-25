package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func protected(t *testing.T) (http.Handler, *Service) {
	t.Helper()
	svc := newTestService(t)
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	mux.HandleFunc("GET /health", ok)
	mux.HandleFunc("GET /meters", ok)
	mux.HandleFunc("PATCH /anomalies/{id}", ok)
	mux.HandleFunc("POST /auth/login", ok)
	return Middleware(svc, "GET /health", "POST /auth/login")(mux), svc
}

func call(h http.Handler, method, path, authz string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMiddlewarePublicRoutes(t *testing.T) {
	h, _ := protected(t)
	for _, c := range [][2]string{{"GET", "/health"}, {"POST", "/auth/login"}} {
		if rec := call(h, c[0], c[1], ""); rec.Code != http.StatusOK {
			t.Errorf("%s %s sin token → %d, quería 200", c[0], c[1], rec.Code)
		}
	}
}

func TestMiddlewareProtectsEverythingElse(t *testing.T) {
	h, _ := protected(t)
	cases := [][2]string{
		{"GET", "/meters"}, {"PATCH", "/anomalies/abc"}, {"GET", "/no-existe"},
		{"POST", "/health"},          // otro método sobre una ruta pública
		{"GET", "/health/"},          // variante con barra
		{"GET", "/health/../meters"}, // intento de colarse con ..
		{"GET", "/auth/login"},       // login es solo POST
	}
	for _, c := range cases {
		rec := call(h, c[0], c[1], "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s sin token → %d, quería 401", c[0], c[1], rec.Code)
		}
	}
	rec := call(h, "GET", "/meters", "")
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("falta WWW-Authenticate")
	}
}

func TestMiddlewareAcceptsValidToken(t *testing.T) {
	h, svc := protected(t)
	tok, _ := svc.Login("demo", "clave-correcta")
	for _, scheme := range []string{"Bearer ", "bearer ", "BEARER "} {
		if rec := call(h, "GET", "/meters", scheme+tok.AccessToken); rec.Code != http.StatusOK {
			t.Errorf("esquema %q → %d", scheme, rec.Code)
		}
	}
	if rec := call(h, "PATCH", "/anomalies/abc", "Bearer "+tok.AccessToken); rec.Code != http.StatusOK {
		t.Errorf("PATCH con token → %d", rec.Code)
	}
}

func TestMiddlewareRejectsBadAuthorization(t *testing.T) {
	h, svc := protected(t)
	tok, _ := svc.Login("demo", "clave-correcta")
	for name, authz := range map[string]string{
		"sin esquema":    tok.AccessToken,
		"esquema Basic":  "Basic " + tok.AccessToken,
		"token vacío":    "Bearer ",
		"token basura":   "Bearer abc.def.ghi",
		"token alterado": "Bearer " + tok.AccessToken + "x",
	} {
		if rec := call(h, "GET", "/meters", authz); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s → %d, quería 401", name, rec.Code)
		}
	}

	svc.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	rec := call(h, "GET", "/meters", "Bearer "+tok.AccessToken)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("token expirado → %d", rec.Code)
	}
	if body := rec.Body.String(); len(body) == 0 || strings.Contains(body, "expired") || strings.Contains(body, "claim") {
		t.Errorf("el motivo técnico no debe salir al cliente: %s", body)
	}
}
