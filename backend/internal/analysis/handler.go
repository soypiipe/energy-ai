package analysis

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/soypiipe/energy-ai/backend/internal/httpx"
)

// uuidPattern valida el id de la ruta antes de tocar la base.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /ai/analyze", h.analyze)
	mux.HandleFunc("GET /ai/analysis/{id}", h.getRun)
}

// runResponse es lo que ve el cliente de una ejecución. El error interno (SQL, rutas, etc.) nunca
// sale: si falló, se devuelve un mensaje genérico y el detalle queda en el log del servidor.
type runResponse struct {
	ID          string          `json:"id"`
	Status      string          `json:"status"`
	CurrentStep *string         `json:"current_step"`
	Error       *string         `json:"error"`
	Summary     json.RawMessage `json:"summary"`
	CreatedAt   time.Time       `json:"created_at"`
	StartedAt   *time.Time      `json:"started_at"`
	FinishedAt  *time.Time      `json:"finished_at"`
}

func toResponse(r Run) runResponse {
	resp := runResponse{
		ID: r.ID, Status: r.Status, CurrentStep: r.CurrentStep, Summary: r.Summary,
		CreatedAt: r.CreatedAt, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt,
	}
	if r.Status == StatusFailed {
		msg := "El análisis falló. Vuelve a intentarlo."
		resp.Error = &msg
	}
	return resp
}

// analyze encola un análisis y responde 202: el trabajo sigue en segundo plano. Si ya hay uno
// PENDING o RUNNING devuelve ese mismo (un doble clic no apila trabajo duplicado).
func (h *Handler) analyze(w http.ResponseWriter, r *http.Request) {
	active, err := h.repo.ActiveRun(r.Context())
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	run := active
	if run == nil {
		created, err := h.repo.Enqueue(r.Context())
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		run = &created
	}
	w.Header().Set("Location", "/ai/analysis/"+run.ID)
	httpx.WriteJSON(w, http.StatusAccepted, toResponse(*run))
}

func (h *Handler) getRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_ID", "id inválido")
		return
	}
	run, err := h.repo.Get(r.Context(), id)
	if errors.Is(err, ErrRunNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "ANALYSIS_NOT_FOUND", "El análisis no existe")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toResponse(run))
}
