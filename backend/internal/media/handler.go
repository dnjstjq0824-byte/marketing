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

// List handles GET /api/media?kind=&product=&business=&from=&to=&page=&size=.
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
	items, err := h.svc.List(q.Get("kind"), int64(product), q.Get("business"), q.Get("from"), q.Get("to"), page, size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list media")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// SignedFile handles GET /api/media/{id}/signed?exp=&sig=: the worker
// download path (W5-3). Public route, signature-checked, no login.
func (h *Handler) SignedFile(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	q := r.URL.Query()
	if !VerifyFileURL(h.svc.FileKey, id, q.Get("exp"), q.Get("sig")) {
		writeError(w, http.StatusForbidden, "bad or expired signature")
		return
	}
	h.File(w, r)
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
// The attachment name follows the W14-5 rule. GET .../download is the
// same handler under the UI_DESIGN name.
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
	d, derr := h.svc.Detail(id)
	name := filepath.Base(m.Path)
	if derr == nil {
		name = d.Download
	}
	w.Header().Set("Content-Type", m.Mime)
	w.Header().Set("X-SHA256", m.SHA256)
	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(name))
	http.ServeContent(w, r, m.Path, time.Time{}, f)
}

// Thumb handles GET /api/media/{id}/thumb: the stored thumbnail, or 404
// when the row has none (callers fall back to /file).
func (h *Handler) Thumb(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	m, f, err := h.svc.OpenThumb(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "no thumbnail")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-SHA256", m.SHA256)
	http.ServeContent(w, r, m.ThumbPath, time.Time{}, f)
}

// LegacyImage handles GET /api/product-images/{id}: pre-library uploads
// behind operator auth (the /uploads/ replacement for legacy bytes).
func (h *Handler) LegacyImage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid image id")
		return
	}
	im, f, err := h.svc.OpenLegacy(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "image not found")
		return
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := f.Read(head)
	head = head[:n]
	if _, err := f.Seek(0, 0); err != nil {
		writeError(w, http.StatusNotFound, "image not found")
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(head))
	http.ServeContent(w, r, im.Path, time.Time{}, f)
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

// GenAI handles POST /api/worker/media/gen: one AI-session generation.
// Workers call it for route=ai media jobs; unverified platforms answer
// 409 so the worker falls back to the local cost-0 path.
func (h *Handler) GenAI(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID int64  `json:"accountId"`
		Kind      string `json:"kind"`
		Prompt    string `json:"prompt"`
		Ratio     string `json:"ratio"`
		ContentID int64  `json:"contentId"`
		ProductID int64  `json:"productId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	m, err := h.svc.GenViaAI(req.AccountID, req.Kind, req.Prompt, req.Ratio, req.ContentID, req.ProductID)
	if err != nil {
		switch {
		case errors.Is(err, ErrGenBadKind), errors.Is(err, ErrGenBadPrompt):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrGenNoAccount):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrGenNotConnected), errors.Is(err, ErrGenNoCap), errors.Is(err, ErrGenUnavailable):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, ErrGenNoPython):
			writeError(w, http.StatusServiceUnavailable, err.Error())
		default:
			writeError(w, http.StatusBadGateway, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": m.ID, "kind": m.Kind, "bytes": m.Bytes, "mime": m.Mime,
		"sha256": m.SHA256, "width": m.Width, "height": m.Height,
	})
}

// Complete handles POST /api/media/uploads/{id}/complete. An optional JSON
// body {kind, prompt, accountId} marks worker-made generations (W14);
// operator uploads omit it and record plain uploads.
func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	var meta struct {
		Kind      string `json:"kind"`
		Prompt    string `json:"prompt"`
		AccountID int64  `json:"accountId"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&meta)
	}
	switch meta.Kind {
	case "", store.MediaKindUpload, store.MediaKindGenImage, store.MediaKindGenVideo, store.MediaKindCard:
	default:
		writeError(w, http.StatusBadRequest, "unknown media kind")
		return
	}
	if len(meta.Prompt) > 2000 {
		writeError(w, http.StatusBadRequest, "prompt too long")
		return
	}
	m, err := h.svc.CompleteFull(r.PathValue("id"), meta.Kind, meta.Prompt, meta.AccountID)
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
