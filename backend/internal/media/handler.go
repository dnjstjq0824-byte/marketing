package media

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"somun/internal/store"
)

// Handler serves chunked uploads.
type Handler struct {
	svc *Service
}

// NewHandler builds the upload handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Init handles POST /api/media/uploads.
func (h *Handler) Init(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	id, total, err := h.svc.Init(req.Name, req.Size, req.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "total": total, "doneParts": 0})
}

// PutPart handles PUT /api/media/uploads/{id}/parts/{n}.
func (h *Handler) PutPart(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid part number")
		return
	}
	done, total, err := h.svc.PutPart(r.PathValue("id"), n, r.Body)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoSession):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrOutOfOrder):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"done": done, "total": total})
}

// List handles GET /api/media?kind=&product=&page=&size=.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	product, _ := strconv.Atoi(q.Get("product"))
	page, _ := strconv.Atoi(q.Get("page"))
	size, _ := strconv.Atoi(q.Get("size"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 48
	}
	items, err := h.svc.List(q.Get("kind"), int64(product), q.Get("business"), page, size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list media")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// Detail handles GET /api/media/{id}.
func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	m, err := h.svc.Detail(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "media not found")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// File handles GET /api/media/{id}/file: original bytes with hash header.
func (h *Handler) File(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	m, f, err := h.svc.Open(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "media not found")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", m.Mime)
	w.Header().Set("X-SHA256", m.SHA256)
	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(filepath.Base(m.Path)))
	http.ServeContent(w, r, m.Path, time.Time{}, f)
}

// Zip handles POST /api/media/zip {ids}: streams a zip of originals.
func (h *Handler) Zip(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids are required")
		return
	}
	if len(req.IDs) > 50 {
		writeError(w, http.StatusBadRequest, "too many files (max 50)")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="somun-media.zip"`)
	if err := h.svc.Zip(w, req.IDs); err != nil {
		// Headers already sent; nothing honest left to write.
		return
	}
}

// Delete handles DELETE /api/media/{id}. Attached files are refused.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	if err := h.svc.Delete(id); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "media not found")
		case errors.Is(err, store.ErrAttached):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot delete media")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": id})
}

// Complete handles POST /api/media/uploads/{id}/complete.
func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	m, err := h.svc.Complete(r.PathValue("id"))
	if err != nil {
		switch {
		case errors.Is(err, ErrNoSession):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrIncomplete):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": m.ID, "kind": m.Kind, "bytes": m.Bytes, "mime": m.Mime,
	})
}

func pathID(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(r.PathValue(key), 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
