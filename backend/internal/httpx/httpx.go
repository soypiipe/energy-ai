// Package httpx agrupa helpers HTTP: respuestas JSON, errores uniformes y middleware.
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"time"
)

// ErrorBody es el formato único de error de la API: {"error": {"code": "...", "message": "..."}}.
type ErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("codificar respuesta JSON", "err", err)
	}
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	var body ErrorBody
	body.Error.Code = code
	body.Error.Message = message
	WriteJSON(w, status, body)
}

// InternalError registra el detalle en el log y devuelve un mensaje genérico:
// nunca se exponen errores internos (SQL, rutas, etc.) al cliente.
func InternalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("error interno", "method", r.Method, "path", r.URL.Path, "err", err)
	WriteError(w, http.StatusInternalServerError, "INTERNAL", "Error interno del servidor")
}

// ---- Middleware ----

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Logger registra método, ruta, estado y duración de cada petición.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", time.Since(start))
	})
}

// Recoverer evita que un panic tumbe el servidor y responde 500.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("panic", "path", r.URL.Path, "value", v)
				WriteError(w, http.StatusInternalServerError, "INTERNAL", "Error interno del servidor")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CORS permite solo los orígenes de la lista blanca (nunca "*" con credenciales).
func CORS(allowed []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(allowed, origin) {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Vary", "Origin")
				h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				h.Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LimitBody limita el tamaño del body para evitar abusos de memoria.
func LimitBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// Chain aplica los middleware en el orden en que se listan (el primero es el más externo).
func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// LocalTime serializa un timestamp SIN zona horaria ("2026-09-12T14:00:00").
// Los datos vienen en hora local de planta sin zona; mostrarlos con "Z" (UTC) sería falso.
type LocalTime time.Time

const localTimeLayout = "2006-01-02T15:04:05"

func (t LocalTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(t).Format(localTimeLayout) + `"`), nil
}
