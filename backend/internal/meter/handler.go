package meter

import (
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/soypiipe/energy-ai/backend/internal/httpx"
)

// meterIDPattern valida el parámetro de ruta antes de tocar la base (p. ej. "M-109").
var meterIDPattern = regexp.MustCompile(`^[A-Z]{1,4}-\d{1,6}$`)

const timeLayout = "2006-01-02T15:04:05"

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

// Register monta las rutas en el mux. Go 1.22+ permite "MÉTODO /ruta/{param}".
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /meters", h.list)
	mux.HandleFunc("GET /meters/{meterId}", h.get)
	mux.HandleFunc("GET /meters/{meterId}/readings", h.readings)
}

// list devuelve los 12 medidores con su resumen. Filtro, búsqueda y orden se hacen en el
// frontend: con 12 filas no se justifica paginar ni filtrar en el servidor.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	meters, err := h.repo.ListSummaries(r.Context())
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": meters})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := meterID(w, r)
	if !ok {
		return
	}
	detail, err := h.repo.GetDetail(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "METER_NOT_FOUND", "El medidor no existe")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, detail)
}

// readings acepta ?from=2026-09-10T00:00:00&to=2026-09-14T23:00:00 (ambos opcionales).
func (h *Handler) readings(w http.ResponseWriter, r *http.Request) {
	id, ok := meterID(w, r)
	if !ok {
		return
	}
	from, err := optionalTime(r, "from")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_FROM", "from debe tener formato "+timeLayout)
		return
	}
	to, err := optionalTime(r, "to")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_TO", "to debe tener formato "+timeLayout)
		return
	}
	if from != nil && to != nil && from.After(*to) {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_RANGE", "from debe ser anterior a to")
		return
	}

	readings, err := h.repo.Readings(r.Context(), id, from, to)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "METER_NOT_FOUND", "El medidor no existe")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"meter_id": id, "data": readings})
}

func meterID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("meterId")
	if !meterIDPattern.MatchString(id) {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_METER_ID", "meterId inválido (ejemplo: M-109)")
		return "", false
	}
	return id, true
}

func optionalTime(r *http.Request, key string) (*time.Time, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse(timeLayout, v)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
