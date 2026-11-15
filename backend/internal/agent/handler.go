package agent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"somun/internal/store"
)

// Handler serves operator agent endpoints plus worker heartbeats.
type Handler struct {
	svc *Service
}

// NewHandler builds the agent handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type registerRequest struct {
	Label      string `json:"label"`
	Lane       string `json:"lane"`
	Host       string `json:"host"`
	Businesses string `json:"businesses"`
	Platforms  string `json:"platforms"`
	Capacity   int    `json:"capacity"`
	AccountID  *int64 `json:"accountId"`
}

// Register handles POST /api/agents. The token shows exactly once.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Capacity == 0 {
		req.Capacity = 1
	}
	a, token, err := h.svc.Register(req.Label, strings.ToUpper(strings.TrimSpace(req.Lane)),
		req.Host, req.Businesses, req.Platforms, req.Capacity, req.AccountID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrDuplicate):
			writeError(w, http.StatusConflict, "agent label already exists")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"agent": a, "token": token})
}

// List handles GET /api/agents with filters and paging.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var f store.AgentFilter
	f.Lane = strings.ToUpper(strings.TrimSpace(q.Get("lane")))
	if s := strings.TrimSpace(q.Get("enabled")); s != "" {
		v := s == "1" || strings.EqualFold(s, "true")
		f.Enabled = &v
	}
	f.Business = strings.TrimSpace(q.Get("business"))
	f.Host = strings.TrimSpace(q.Get("host"))
	f.Query = strings.TrimSpace(q.Get("q"))
	limit, err := atoiBounded(q.Get("limit"), 50, 1, 200)
	if err != nil {
		writeError(w, http.StatusBadRequest, "limit must be 1..200")
		return
	}
	offset, err := atoiBounded(q.Get("offset"), 0, 0, 1<<30)
	if err != nil {
		writeError(w, http.StatusBadRequest, "offset must be >= 0")
		return
	}
	items, total, err := h.svc.List(f, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list agents")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

// Detail handles GET /api/agents/{id}: the agent plus recent jobs.
func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	a, jobs, err := h.svc.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot load agent")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent": a, "jobs": jobs})
}

type updateRequest struct {
	Lane       *string `json:"lane"`
	Host       *string `json:"host"`
	Businesses *string `json:"businesses"`
	Platforms  *string `json:"platforms"`
	Capacity   *int    `json:"capacity"`
	Enabled    *bool   `json:"enabled"`
	AccountID  *int64  `json:"accountId"`
}

// Update handles PATCH /api/agents/{id}. Label is immutable.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	cur, _, err := h.svc.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot load agent")
		return
	}
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	lane, host, biz, plats := cur.Lane, cur.Host, cur.Businesses, cur.Platforms
	capacity, enabled, accountID := cur.Capacity, cur.Enabled, cur.AccountID
	if req.Lane != nil {
		lane = strings.ToUpper(strings.TrimSpace(*req.Lane))
	}
	if req.Host != nil {
		host = strings.TrimSpace(*req.Host)
	}
	if req.Businesses != nil {
		biz = strings.TrimSpace(*req.Businesses)
	}
	if req.Platforms != nil {
		plats = strings.TrimSpace(*req.Platforms)
	}
	if req.Capacity != nil {
		capacity = *req.Capacity
	}
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if req.AccountID != nil {
		accountID = req.AccountID
	}
	a, err := h.svc.Update(id, lane, host, biz, plats, capacity, enabled, accountID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// Delete handles DELETE /api/agents/{id}. Live claims block it with 409.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	if err := h.svc.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": id})
}

// Rotate handles POST /api/agents/{id}/rotate-token.
func (h *Handler) Rotate(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	token, err := h.svc.Rotate(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot rotate token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token})
}

type bulkRequest struct {
	IDs    []int64 `json:"ids"`
	Action string  `json:"action"`
	Lane   string  `json:"lane"`
}

// Bulk handles POST /api/agents/bulk.
func (h *Handler) Bulk(w http.ResponseWriter, r *http.Request) {
	var req bulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	n, err := h.svc.Bulk(req.IDs, strings.ToLower(strings.TrimSpace(req.Action)),
		strings.ToUpper(strings.TrimSpace(req.Lane)))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": n})
}

// Stats handles GET /api/agents/stats.
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Stats(time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load agent stats")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

type heartbeatRequest struct {
	SessionState string `json:"sessionState"`
	ActiveJobs   int    `json:"activeJobs"`
	Version      string `json:"version"`
}

// Heartbeat handles POST /api/worker/heartbeat (agent token only).
func (h *Handler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	a, ok := AgentFrom(r.Context())
	if !ok || a == nil {
		writeError(w, http.StatusUnauthorized, "agent token required")
		return
	}
	var req heartbeatRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	enabled, err := h.svc.Heartbeat(a, strings.TrimSpace(req.SessionState))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot record heartbeat")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "enabled": enabled})
}

// JobHeartbeat handles POST /api/worker/jobs/{id}/heartbeat (own claims only).
func (h *Handler) JobHeartbeat(w http.ResponseWriter, r *http.Request) {
	a, ok := AgentFrom(r.Context())
	if !ok || a == nil {
		writeError(w, http.StatusUnauthorized, "agent token required")
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	lease, err := h.svc.JobHeartbeat(a, id)
	if err != nil {
		if errors.Is(err, store.ErrLeaseDenied) {
			writeError(w, http.StatusConflict, "job is not claimed by this agent")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot extend lease")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"leaseUntil": lease})
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func atoiBounded(raw string, fallback, min, max int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if v < min || v > max {
		return 0, strconv.ErrRange
	}
	return v, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
