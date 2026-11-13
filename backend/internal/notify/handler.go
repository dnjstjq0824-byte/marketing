package notify

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler serves the operator notification endpoints.
type Handler struct {
	svc *Service
}

// NewHandler builds the notification handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// List handles GET /api/notifications?unread=&limit=&offset=.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	unread := q.Get("unread") == "1" || q.Get("unread") == "true"
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
	items, n, err := h.svc.List(unread, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list notifications")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "unread": n})
}

// Read handles POST /api/notifications/{id}/read.
func (h *Handler) Read(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid notification id")
		return
	}
	ok, err := h.svc.Read(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot mark read")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "notification not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ReadAll handles POST /api/notifications/read-all.
func (h *Handler) ReadAll(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.ReadAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot mark all read")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated": n})
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
