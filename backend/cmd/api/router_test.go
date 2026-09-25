package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/soypiipe/energy-ai/backend/internal/analysis"
	"github.com/soypiipe/energy-ai/backend/internal/auth"
	"github.com/soypiipe/energy-ai/backend/internal/meter"
	"github.com/soypiipe/energy-ai/backend/internal/pgtest"
	"github.com/soypiipe/energy-ai/backend/internal/seed"
)

// Prueba el router completo con base real: qué exige token y qué no.
func TestRouterAuthProtection(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	if _, err := seed.Run(ctx, pool, "../../../data"); err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("clave-de-prueba"), bcrypt.MinCost)
	svc, err := auth.NewService("demo", string(hash), strings.Repeat("s", 32), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	h := newRouter(pool, []string{"http://localhost:5173"}, svc, meter.NewRepository(pool), analysis.NewRepository(pool))

	do := func(method, path, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = "203.0.113.1:1"
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	const id = "00000000-0000-0000-0000-000000000000"

	// documentación pública: la especificación y la interfaz Swagger UI cargan sin token
	for _, p := range []string{"/openapi.yaml", "/docs/", "/docs/swagger-ui.css", "/docs/init.js"} {
		if rec := do("GET", p, "", ""); rec.Code != 200 || rec.Body.Len() == 0 {
			t.Errorf("GET %s sin token → %d", p, rec.Code)
		}
	}
	if rec := do("GET", "/docs", "", ""); rec.Code != http.StatusMovedPermanently {
		t.Errorf("/docs → %d, quería redirección a /docs/", rec.Code)
	}
	if rec := do("GET", "/openapi.yaml", "", ""); !strings.Contains(rec.Body.String(), "openapi: 3.0.3") {
		t.Error("la especificación no es OpenAPI 3")
	}

	// públicas: sin token
	if rec := do("GET", "/health", "", ""); rec.Code != 200 {
		t.Errorf("/health → %d", rec.Code)
	}

	// todas las demás rutas de la API (docs/DESIGN.md §7): sin token → 401
	protectedRoutes := [][2]string{
		{"GET", "/meters"}, {"GET", "/meters/M-109"}, {"GET", "/meters/M-109/readings"},
		{"GET", "/anomalies"}, {"GET", "/anomalies/" + id}, {"PATCH", "/anomalies/" + id},
		{"POST", "/ai/analyze"}, {"GET", "/ai/analysis/" + id}, {"GET", "/dashboard/summary"},
	}
	for _, r := range protectedRoutes {
		if rec := do(r[0], r[1], `{"status":"OPEN"}`, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s sin token → %d, quería 401", r[0], r[1], rec.Code)
		}
	}
	// login incorrecto
	if rec := do("POST", "/auth/login", `{"username":"demo","password":"mala"}`, ""); rec.Code != 401 {
		t.Errorf("login incorrecto → %d", rec.Code)
	}

	// login correcto → con el token las rutas responden
	rec := do("POST", "/auth/login", `{"username":"demo","password":"clave-de-prueba"}`, "")
	var tok auth.Token
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &tok) != nil {
		t.Fatalf("login = %d %s", rec.Code, rec.Body)
	}
	for _, r := range [][2]string{{"GET", "/meters"}, {"GET", "/meters/M-109"}, {"GET", "/anomalies"}, {"GET", "/dashboard/summary"}} {
		if rec := do(r[0], r[1], "", tok.AccessToken); rec.Code != 200 {
			t.Errorf("%s %s con token → %d", r[0], r[1], rec.Code)
		}
	}
	if rec := do("POST", "/ai/analyze", "", tok.AccessToken); rec.Code != http.StatusAccepted {
		t.Errorf("POST /ai/analyze con token → %d", rec.Code)
	}

	// el preflight CORS del navegador no lleva token y debe responderse igual
	req := httptest.NewRequest("OPTIONS", "/meters", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	pre := httptest.NewRecorder()
	h.ServeHTTP(pre, req)
	if pre.Code != http.StatusNoContent || pre.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("preflight → %d %v", pre.Code, pre.Header())
	}
}
