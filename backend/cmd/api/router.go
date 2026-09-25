package main

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/soypiipe/energy-ai/backend/internal/analysis"
	"github.com/soypiipe/energy-ai/backend/internal/auth"
	"github.com/soypiipe/energy-ai/backend/internal/dashboard"
	"github.com/soypiipe/energy-ai/backend/internal/httpx"
	"github.com/soypiipe/energy-ai/backend/internal/meter"
)

// publicRoutes son las únicas rutas que no exigen token. Cualquier ruta nueva queda protegida por defecto.
var publicRoutes = []string{"GET /health", "POST /auth/login"}

// newRouter monta todas las rutas y los middleware. Está separado de run() para poder probar el
// conjunto completo (incluida la protección con JWT) sin arrancar un servidor.
func newRouter(pool *pgxpool.Pool, allowedOrigins []string, authSvc *auth.Service, meters *meter.Repository, analyses *analysis.Repository) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			httpx.WriteError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "Base de datos no disponible")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	auth.NewHandler(authSvc).Register(mux)
	meter.NewHandler(meters).Register(mux)
	analysis.NewHandler(analyses).Register(mux)
	dashboard.NewHandler(meters, analyses).Register(mux)

	// Orden (de fuera hacia dentro): CORS va antes que auth para que los preflight OPTIONS del navegador,
	// que no llevan token, se respondan sin autenticar.
	return httpx.Chain(mux,
		httpx.Recoverer,
		httpx.Logger,
		httpx.CORS(allowedOrigins),
		httpx.LimitBody(1<<20), // 1 MB
		auth.Middleware(authSvc, publicRoutes...),
	)
}
