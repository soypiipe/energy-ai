package auth

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/soypiipe/energy-ai/backend/internal/httpx"
)

type Handler struct {
	svc     *Service
	limiter *attemptLimiter
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc, limiter: newAttemptLimiter(loginMaxAttempts, loginWindow)}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/login", h.login)
}

// Límite anti fuerza bruta: intentos de login por IP en una ventana fija.
const (
	loginMaxAttempts = 10
	loginWindow      = time.Minute
)

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !h.limiter.allow(ip) {
		w.Header().Set("Retry-After", "60")
		httpx.WriteError(w, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", "Demasiados intentos. Espera un minuto.")
		return
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || body.Username == "" || body.Password == "" {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_BODY", `El cuerpo debe ser {"username": "...", "password": "..."}`)
		return
	}

	token, err := h.svc.Login(body.Username, body.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		// Mismo mensaje para usuario o contraseña incorrectos: no se revela cuál falló.
		httpx.WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Usuario o contraseña incorrectos")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, token)
}

// clientIP usa la IP de la conexión. No se confía en X-Forwarded-For: cualquiera podría falsearlo
// para esquivar el límite. Detrás de un proxy propio habría que configurarlo explícitamente.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// attemptLimiter cuenta intentos por clave en una ventana fija. En memoria: basta para un solo proceso.
type attemptLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	now    func() time.Time
	seen   map[string]*window
}

type window struct {
	start time.Time
	count int
}

func newAttemptLimiter(max int, w time.Duration) *attemptLimiter {
	return &attemptLimiter{max: max, window: w, now: time.Now, seen: map[string]*window{}}
}

func (l *attemptLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	// Barrido de ventanas vencidas para que el mapa no crezca sin límite.
	for k, v := range l.seen {
		if now.Sub(v.start) >= l.window {
			delete(l.seen, k)
		}
	}
	v, ok := l.seen[key]
	if !ok {
		l.seen[key] = &window{start: now, count: 1}
		return true
	}
	v.count++
	return v.count <= l.max
}
