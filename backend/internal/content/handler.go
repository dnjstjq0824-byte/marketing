package content

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"somun/internal/auth"
	"somun/internal/store"
)

// Handler serves the content read endpoints, deletes, and the S3-D
// publish confirmation. Manual edit/schedule/record endpoints were
// removed in W8 (§3-3); the state machine lives in the service layer.
type Handler struct {
	svc   *Service
	store *store.Store
}

// NewHandler builds the content handler.
func NewHandler(svc *Service, st *store.Store) *Handler {
	return &Handler{svc: svc, store: st}
}

// List handles GET /api/contents?status=&business=.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListContents(
		strings.TrimSpace(r.URL.Query().Get("status")),
		strings.TrimSpace(r.URL.Query().Get("business")), 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list contents")
		return
	}
	if items == nil {
		items = []*store.Content{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"contents": items})
}

// Detail handles GET /api/contents/{id}.
func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid content id")
		return
	}
	c, err := h.store.GetContent(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "content not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot load content")
		return
	}
	if revs, err := h.store.ListRevisionIDs(id); err == nil && len(revs) > 0 {
		c.Revisions = revs
	}
	writeJSON(w, http.StatusOK, c)
}

// ListReviews handles GET /api/contents/{id}/reviews.
func (h *Handler) ListReviews(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid content id")
		return
	}
	if _, err := h.store.GetContent(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "content not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot load content")
		return
	}
	items, err := h.store.ListReviewDecisions(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list reviews")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviews": items})
}

// Delete handles DELETE /api/contents/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid content id")
		return
	}
	if err := h.svc.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "content not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot delete content")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": id})
}

// ConfirmPublished handles POST /api/works/{id}/posts/{postId}/confirm-published
// (S3-D "올라가 있음 / 안 올라감"). The post must belong to the work.
func (h *Handler) ConfirmPublished(w http.ResponseWriter, r *http.Request) {
	workID, err := pathID(r)
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
		Published *bool  `json:"published"`
		URL       string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Published == nil {
		writeError(w, http.StatusBadRequest, "published is required")
		return
	}
	c, err := h.store.GetContent(postID)
	if err != nil || c.ProductID != workID {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	res, err := h.svc.ConfirmPublished(postID, *req.Published, strings.TrimSpace(req.URL))
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "post not found")
		case errors.Is(err, store.ErrDuplicate):
			writeError(w, http.StatusConflict, "publish already recorded")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	action := store.AuditConfirmMissing
	verb := "안 올라감 확인"
	if *req.Published {
		action = store.AuditConfirmPublished
		verb = "올라가 있음 확인"
	}
	_, _ = h.store.AppendAudit(auth.OperatorName(r.Context()), action, "post", postID, workID, verb)
	writeJSON(w, http.StatusOK, res)
}

// ListPublishLogs handles GET /api/publish-logs.
func (h *Handler) ListPublishLogs(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListPublishLogs(200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list publish logs")
		return
	}
	if items == nil {
		items = []*store.PublishLog{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": items})
}

// PublishTrend handles GET /api/publish-trend?days= (console chart).
func (h *Handler) PublishTrend(w http.ResponseWriter, r *http.Request) {
	days := 14
	if q := strings.TrimSpace(r.URL.Query().Get("days")); q != "" {
		n, err := strconv.Atoi(q)
		if err != nil || n < 1 || n > 90 {
			writeError(w, http.StatusBadRequest, "days must be 1..90")
			return
		}
		days = n
	}
	items, err := h.store.PublishTrend(days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load publish trend")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"days": items})
}

// ListMetrics handles GET /api/metrics.
func (h *Handler) ListMetrics(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListMetrics(200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list metrics")
		return
	}
	if items == nil {
		items = []*store.Metric{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"metrics": items})
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
