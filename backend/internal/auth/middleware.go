package auth

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/soypiipe/energy-ai/backend/internal/httpx"
)

// Middleware exige un token válido en todas las rutas salvo las públicas. Es una lista blanca:
// una ruta nueva queda protegida por defecto y hay que declarar explícitamente que es pública.
//
// public son pares exactos "MÉTODO /ruta" (p. ej. "GET /health"). La comparación es literal sobre la
// ruta recibida, así que variantes como "/health/" o "/health/../meters" NO son públicas.
func Middleware(svc *Service, public ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(public))
	for _, p := range public {
		allowed[p] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if allowed[r.Method+" "+r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			token, ok := bearerToken(r)
			if !ok {
				unauthorized(w, "Falta el token de acceso")
				return
			}
			if _, err := svc.Verify(token); err != nil {
				// El motivo (expirado, firma inválida...) va al log; al cliente solo un mensaje genérico.
				slog.Info("token rechazado", "path", r.URL.Path, "err", err)
				unauthorized(w, "Token inválido o expirado")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// bearerToken extrae el token de "Authorization: Bearer <token>" (el esquema no distingue mayúsculas).
func bearerToken(r *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	token = strings.TrimSpace(token)
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}

func unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="energy-ai"`)
	httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", message)
}
