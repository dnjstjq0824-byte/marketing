package product

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"somun/internal/config"
	"somun/internal/store"
)

// Handler serves product endpoints.
type Handler struct {
	svc       *Service
	store     *store.Store
	uploadDir string
}

// NewHandler builds the product handler.
func NewHandler(svc *Service, st *store.Store, uploadDir string) *Handler {
	return &Handler{svc: svc, store: st, uploadDir: uploadDir}
}

// Create handles POST /api/products (multipart with images, or JSON).
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(int64(config.MaxUploadMB) << 20 * (config.MaxImagesPerProduct + 1)); err != nil {
			writeError(w, http.StatusBadRequest, "cannot parse multipart body")
			return
		}
		in.SourceURL = r.FormValue("sourceUrl")
		in.ExitType = r.FormValue("exitType")
		in.Business = r.FormValue("business")
		if raw := r.FormValue("priority"); raw != "" {
			p, err := strconv.Atoi(raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, ErrBadPriority.Error())
				return
			}
			in.Priority = p
		}
		files := r.MultipartForm.File["images"]
		if len(files) > config.MaxImagesPerProduct {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("at most %d images", config.MaxImagesPerProduct))
			return
		}
		for i, fh := range files {
			if fh.Size > int64(config.MaxUploadMB)<<20 {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("image %d exceeds %dMB", i+1, config.MaxUploadMB))
				return
			}
			path, err := h.saveUpload(fh)
			if err != nil {
				writeError(w, http.StatusBadRequest, "cannot save upload: "+err.Error())
				return
			}
			in.Images = append(in.Images, path)
		}
	} else {
		var req struct {
			SourceURL string `json:"sourceUrl"`
			ExitType  string `json:"exitType"`
			Priority  int    `json:"priority"`
			Business  string `json:"business"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		in = CreateInput{SourceURL: req.SourceURL, ExitType: req.ExitType, Priority: req.Priority, Business: req.Business}
	}
	res, err := h.svc.Create(in)
	if err != nil {
		switch {
		case errors.Is(err, ErrBadExitType), errors.Is(err, ErrBadURL), errors.Is(err, ErrBadPriority):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot create product")
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"product": res.Product, "analyzeJob": res.Job})
}

func (h *Handler) saveUpload(fh *multipart.FileHeader) (string, error) {
	f, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()
	return saveUploadedFile(h.uploadDir, f, fh.Filename)
}

// List handles GET /api/products?business=.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListProducts(200, strings.TrimSpace(r.URL.Query().Get("business")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list products")
		return
	}
	if items == nil {
		items = []*store.Product{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"products": items})
}

// Detail handles GET /api/products/{id}.
func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	d, err := h.svc.GetDetail(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "product not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot load product")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// Delete handles DELETE /api/products/{id}: hard delete with cascade plus
// upload file cleanup.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	paths, err := h.svc.Delete(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "product not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot delete product")
		return
	}
	removed := 0
	for _, p := range paths {
		if err := os.Remove(filepath.Join(h.uploadDir, filepath.Base(p))); err == nil {
			removed++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": id, "imagesRemoved": removed})
}

type updateRequest struct {
	Name      string `json:"name"`
	SourceURL string `json:"sourceUrl"`
	ExitType  string `json:"exitType"`
	Priority  int    `json:"priority"`
	Business  string `json:"business"`
	Industry  string `json:"industry"`
}

// Update handles PATCH /api/products/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	p, err := h.svc.Update(id, UpdateInput{
		Name: strings.TrimSpace(req.Name), SourceURL: strings.TrimSpace(req.SourceURL),
		ExitType: req.ExitType, Priority: req.Priority, Business: req.Business,
		Industry: req.Industry,
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "product not found")
		case errors.Is(err, ErrBadExitType), errors.Is(err, ErrBadURL), errors.Is(err, ErrBadPriority), errors.Is(err, ErrBadIndustry):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot update product")
		}
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// DeleteImage handles DELETE /api/products/{id}/images/{imageId}.
func (h *Handler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	imageID, err := pathID(r, "imageId")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid image id")
		return
	}
	path, err := h.svc.DeleteImage(id, imageID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "image not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot delete image")
		return
	}
	_ = os.Remove(filepath.Join(h.uploadDir, filepath.Base(path)))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": imageID})
}

func saveUploadedFile(uploadDir string, file io.Reader, filename string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
	default:
		return "", fmt.Errorf("unsupported image type %q", ext)
	}
	stamp := time.Now().Format("20060102-150405.000000")
	safe := fmt.Sprintf("%s-%s%s", stamp, sanitizeName(strings.TrimSuffix(filepath.Base(filename), ext)), ext)
	dst := filepath.Join(uploadDir, safe)
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		return "", err
	}
	return safe, nil
}

func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if len(out) > 40 {
		out = out[:40]
	}
	if out == "" {
		out = "img"
	}
	return out
}

func pathID(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
