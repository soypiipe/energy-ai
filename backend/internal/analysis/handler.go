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
	mux.HandleFunc("GET /anomalies", h.listAnomalies)
	mux.HandleFunc("GET /anomalies/{id}", h.getAnomaly)
	mux.HandleFunc("PATCH /anomalies/{id}", h.patchAnomaly)
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

// listAnomalies acepta ?meter_id=M-109&type=REAL_ANOMALY&severity=HIGH&status=OPEN (todos opcionales).
func (h *Handler) listAnomalies(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := AnomalyFilter{MeterID: q.Get("meter_id"), Type: q.Get("type"), Severity: q.Get("severity"), Status: q.Get("status")}
	switch {
	case f.MeterID != "" && !meterIDPattern.MatchString(f.MeterID):
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_METER_ID", "meter_id inválido (ejemplo: M-109)")
		return
	case f.Type != "" && !validTypes[f.Type]:
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_TYPE", "type inválido")
		return
	case f.Severity != "" && !validSeverities[f.Severity]:
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_SEVERITY", "severity inválido")
		return
	case f.Status != "" && !validAnomalyStatus(f.Status):
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_STATUS", "status inválido")
		return
	}
	analysisID, list, err := h.repo.ListAnomalies(r.Context(), f)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"analysis_id": nilIfEmpty(analysisID), "data": list})
}

func (h *Handler) getAnomaly(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_ID", "id inválido")
		return
	}
	a, err := h.repo.GetAnomaly(r.Context(), id)
	if errors.Is(err, ErrAnomalyNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "ANOMALY_NOT_FOUND", "La anomalía no existe")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a)
}

// patchAnomaly cambia solo el estado: {"status": "ACKNOWLEDGED"}. Cualquier otro campo se rechaza.
func (h *Handler) patchAnomaly(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_ID", "id inválido")
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_BODY", `El cuerpo debe ser {"status": "OPEN|ACKNOWLEDGED|RESOLVED"}`)
		return
	}
	if !validAnomalyStatus(body.Status) {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_STATUS", "status debe ser OPEN, ACKNOWLEDGED o RESOLVED")
		return
	}
	a, err := h.repo.UpdateAnomalyStatus(r.Context(), id, body.Status)
	if errors.Is(err, ErrAnomalyNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "ANOMALY_NOT_FOUND", "La anomalía no existe")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a)
}

var (
	validTypes      = map[string]bool{"REAL_ANOMALY": true, "EXPLAINABLE_ANOMALY": true, "FALSE_POSITIVE": true, "DATA_QUALITY": true}
	validSeverities = map[string]bool{"LOW": true, "MEDIUM": true, "HIGH": true}
	meterIDPattern  = regexp.MustCompile(`^[A-Z]{1,4}-\d{1,6}$`)
)

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
