package executor

import (
	"encoding/json"
	"net/http"

	"somun/internal/store"
)

// Handler serves the operator executor-config endpoints.
type Handler struct {
	svc *Service
}

// NewHandler builds the executor handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// GetConfig handles GET /api/executor/config.
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.svc.Config()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load executor config")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

type configRequest struct {
	HeartbeatMin   int `json:"heartbeatMin"`
	MinIntervalMin int `json:"minIntervalMin"`
	MaxActive      int `json:"maxActive"`
}

// SetConfig handles POST /api/executor/config.
func (h *Handler) SetConfig(w http.ResponseWriter, r *http.Request) {
	var req configRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	cfg, err := h.svc.SetConfig(store.ExecutorConfig{
		HeartbeatMin: req.HeartbeatMin, MinIntervalMin: req.MinIntervalMin, MaxActive: req.MaxActive,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
