package works

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"somun/internal/store"
)

// Handler serves the works API (S2/S3/S3-D).
type Handler struct {
	svc      *Service
	store    *store.Store
	uploadDir string
}

// NewHandler builds the works handler.
func NewHandler(svc *Service, st *store.Store, uploadDir string) *Handler {
	return &Handler{svc: svc, store: st, uploadDir: uploadDir}
}

// Create handles POST /api/works.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	id, err := h.svc.Create(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// List handles GET /api/works?tab=&q=&business=&page=&size=.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	size, _ := strconv.Atoi(q.Get("size"))
	items, err := h.store.ListWorks(q.Get("tab"), q.Get("q"), q.Get("business"), page, size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list works")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// Detail handles GET /api/works/{id}.
func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid work id")
		return
	}
	d, err := h.store.WorkDetailByID(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "work not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot load work")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// Pause handles POST /api/works/{id}/pause.
func (h *Handler) Pause(w http.ResponseWriter, r *http.Request) {
	h.setHeld(w, r, true)
}

// Resume handles POST /api/works/{id}/resume.
func (h *Handler) Resume(w http.ResponseWriter, r *http.Request) {
	h.setHeld(w, r, false)
}

func (h *Handler) setHeld(w http.ResponseWriter, r *http.Request, held bool) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid work id")
		return
	}
	if _, err := h.store.GetProduct(id); err != nil {
		writeError(w, http.StatusNotFound, "work not found")
		return
	}
	n, err := h.store.SetWorkHeld(id, held)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot update work")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"held": held, "count": n})
}

// DeleteWork handles DELETE /api/works/{id}.
func (h *Handler) DeleteWork(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid work id")
		return
	}
	paths, err := h.svc.Delete(id)
	if err != nil {
		if errors.Is(err, ErrNoAccount) {
			writeError(w, http.StatusNotFound, "work not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot delete work")
		return
	}
	for _, p := range paths {
		os.Remove(filepath.Join(h.uploadDir, p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": id})
}

// HoldPost handles POST /api/works/{id}/posts/{postId}/hold.
func (h *Handler) HoldPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid work id")
		return
	}
	postID, err := strconv.ParseInt(r.PathValue("postId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid post id")
		return
	}
	var req struct {
		Held bool `json:"held"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.svc.HoldPost(id, postID, req.Held); err != nil {
		if errors.Is(err, ErrNoAccount) {
			writeError(w, http.StatusNotFound, "work not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot update post")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"held": req.Held})
}

// RegeneratePost handles POST /api/works/{id}/posts:generate.
func (h *Handler) RegeneratePost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid work id")
		return
	}
	var req struct {
		PostID int64 `json:"postId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PostID <= 0 {
		writeError(w, http.StatusBadRequest, "postId is required")
		return
	}
	jobID, err := h.svc.RegeneratePost(id, req.PostID)
	if err != nil {
		if errors.Is(err, ErrNoAccount) {
			writeError(w, http.StatusNotFound, "work not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot regenerate")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"jobId": jobID})
}

// AILog handles GET /api/works/{id}/ai-log.
func (h *Handler) AILog(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid work id")
		return
	}
	if _, err := h.store.GetProduct(id); err != nil {
		writeError(w, http.StatusNotFound, "work not found")
		return
	}
	items, err := h.store.WorkAILog(id, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load log")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
