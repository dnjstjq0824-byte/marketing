package session

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// Handler serves slot/session endpoints.
type Handler struct {
	svc *Service
}

// NewHandler builds the session handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type registerRequest struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Owner    string `json:"owner"`
	Role     string `json:"role"`
	Business string `json:"business"`
}

// RegisterAccount handles POST /api/session/accounts.
func (h *Handler) RegisterAccount(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	a, err := h.svc.RegisterAccount(strings.TrimSpace(req.Name), strings.TrimSpace(req.Platform),
		strings.TrimSpace(req.Owner), strings.TrimSpace(req.Role), strings.TrimSpace(req.Business))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

// ListAccounts handles GET /api/session/accounts.
func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	items := h.svc.ListAccounts()
	if items == nil {
		items = []*Account{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": items})
}

type updateRequest struct {
	SessionState string `json:"sessionState"`
	Usage        string `json:"usage"`
}

// UpdateAccount handles PATCH /api/session/accounts/{id}.
func (h *Handler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	a, err := h.svc.SetSessionState(id, strings.TrimSpace(req.SessionState), strings.TrimSpace(req.Usage))
	if err != nil {
		switch {
		case errors.Is(err, ErrNoAccount):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrBadState):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot update session")
		}
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// CheckNow handles POST /api/session/check (light inspection skeleton).
func (h *Handler) CheckNow(w http.ResponseWriter, r *http.Request) {
	n := h.svc.CheckNow()
	writeJSON(w, http.StatusOK, map[string]any{"checked": n})
}

// Wake handles POST /api/session/accounts/{id}/wake (assigned sessions only).
func (h *Handler) Wake(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}
	a, err := h.svc.Wake(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// ReloginQueue handles GET /api/session/relogin-queue.
func (h *Handler) ReloginQueue(w http.ResponseWriter, r *http.Request) {
	items := h.svc.ReloginQueue()
	if items == nil {
		items = []*ReloginItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ListSlots handles GET /api/session/slots.
func (h *Handler) ListSlots(w http.ResponseWriter, r *http.Request) {
	items := h.svc.ListSlots()
	if items == nil {
		items = []*Slot{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"slots": items})
}

type slotRequest struct {
	Label     string `json:"label"`
	AgentName string `json:"agentName"`
}

// AddSlot handles POST /api/session/slots.
func (h *Handler) AddSlot(w http.ResponseWriter, r *http.Request) {
	var req slotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	sl, err := h.svc.AddSlot(strings.TrimSpace(req.Label), strings.TrimSpace(req.AgentName))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sl)
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
