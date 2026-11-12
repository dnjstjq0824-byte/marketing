package content

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"somun/internal/content/policy"
	"somun/internal/store"
)

// Handler serves content, publish-queue, publish-log, and metric endpoints.
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

type updateRequest struct {
	Title   *string `json:"title"`
	Body    *string `json:"body"`
	Status  *string `json:"status"`
	Persona *bool   `json:"virtualPersona"`
}

// Update handles PATCH /api/contents/{id}: body edits, review transitions,
// and the virtual-persona flag. Disclosure cannot be edited through this API.
// The queued->published step is structurally valid but blocked here: real
// publishing is tranche 2, and nothing may display as published before that.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid content id")
		return
	}
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	cur, err := h.store.GetContent(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "content not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot load content")
		return
	}
	if req.Title != nil || req.Body != nil {
		title, body := cur.Title, cur.Body
		if req.Title != nil {
			title = *req.Title
		}
		if req.Body != nil {
			body = *req.Body
		}
		if _, err := h.svc.EditBody(id, title, body); err != nil {
			writeError(w, http.StatusInternalServerError, "cannot save content")
			return
		}
	}
	if req.Persona != nil {
		if _, err := h.svc.SetPersona(id, *req.Persona); err != nil {
			writeError(w, http.StatusInternalServerError, "cannot set persona flag")
			return
		}
	}
	if req.Status != nil {
		to := strings.TrimSpace(*req.Status)
		if to == policy.StatusPublished {
			writeError(w, http.StatusConflict, "actual publishing is tranche 2; status stays queued")
			return
		}
		if _, err := h.svc.Transition(id, to, ActorPlatform); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	updated, err := h.store.GetContent(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load content")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

type publishRecordRequest struct {
	URL string `json:"url"`
}

// RecordPublish handles POST /api/contents/{id}/publish-record.
func (h *Handler) RecordPublish(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid content id")
		return
	}
	var req publishRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	c, err := h.svc.RecordManualPublish(id, strings.TrimSpace(req.URL))
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "content not found")
		case errors.Is(err, store.ErrDuplicate):
			writeError(w, http.StatusConflict, "publish already recorded")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, c)
}

type scheduleRequest struct {
	Account     string `json:"account"`
	Platform    string `json:"platform"`
	ScheduledAt string `json:"scheduledAt"`
}

// Schedule handles POST /api/contents/{id}/schedule.
func (h *Handler) Schedule(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid content id")
		return
	}
	var req scheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	res, err := h.svc.Schedule(id, ScheduleInput{
		Account: req.Account, Platform: req.Platform, ScheduledAt: req.ScheduledAt,
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "content not found")
		case errors.Is(err, ErrPastSchedule), errors.Is(err, ErrNoAccount), errors.Is(err, ErrUnknownPlatform):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// PublishQueue handles GET /api/publish-queue?group=official_api|manual.
// group is required: the two channel groups are never served mixed.
func (h *Handler) PublishQueue(w http.ResponseWriter, r *http.Request) {
	group := strings.TrimSpace(r.URL.Query().Get("group"))
	if group != policy.GroupOfficialAPI && group != policy.GroupManual {
		writeError(w, http.StatusBadRequest, "group must be official_api or manual")
		return
	}
	items, err := h.store.ListQueuedByGroup(group)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list publish queue")
		return
	}
	if items == nil {
		items = []*store.Content{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"group": group, "items": items})
}

type publishLogRequest struct {
	ContentID int64  `json:"contentId"`
	Channel   string `json:"channel"`
	Account   string `json:"account"`
	Status    string `json:"status"`
	Note      string `json:"note"`
}

// CreatePublishLog handles POST /api/publish-logs (plan records only).
func (h *Handler) CreatePublishLog(w http.ResponseWriter, r *http.Request) {
	var req publishLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	rec, err := h.svc.RecordPublishPlan(req.ContentID, strings.TrimSpace(req.Channel),
		strings.TrimSpace(req.Account), strings.TrimSpace(req.Status), req.Note)
	if err != nil {
		switch {
		case errors.Is(err, ErrBadPublishLog):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, store.ErrDuplicate):
			writeError(w, http.StatusConflict, "duplicate publish record for content+channel+account")
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "content not found")
		default:
			writeError(w, http.StatusInternalServerError, "cannot record publish log")
		}
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

type publishLogUpdateRequest struct {
	Status *string `json:"status"`
	Note   *string `json:"note"`
}

// UpdatePublishLog handles PATCH /api/publish-logs/{id}.
func (h *Handler) UpdatePublishLog(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid log id")
		return
	}
	var req publishLogUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Status != nil {
		trimmed := strings.TrimSpace(*req.Status)
		req.Status = &trimmed
	}
	rec, err := h.svc.UpdatePublishPlan(id, req.Status, req.Note)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "publish log not found")
		case errors.Is(err, ErrBadPublishLog):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot update publish log")
		}
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

// DeletePublishLog handles DELETE /api/publish-logs/{id}.
func (h *Handler) DeletePublishLog(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid log id")
		return
	}
	if err := h.store.DeletePublishLog(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "publish log not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot delete publish log")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": id})
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

type metricRequest struct {
	ContentID *int64 `json:"contentId"`
	Channel   string `json:"channel"`
	Exposures *int64 `json:"exposures"`
	Inflows   *int64 `json:"inflows"`
	Orders    *int64 `json:"orders"`
	Inquiries *int64 `json:"inquiries"`
	Note      string `json:"note"`
}

// CreateMetric handles POST /api/metrics.
func (h *Handler) CreateMetric(w http.ResponseWriter, r *http.Request) {
	var req metricRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	m, err := h.svc.RecordMetric(MetricInput{
		ContentID: req.ContentID, Channel: strings.TrimSpace(req.Channel),
		Exposures: req.Exposures, Inflows: req.Inflows, Orders: req.Orders,
		Inquiries: req.Inquiries, Note: req.Note,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot record metric")
		return
	}
	writeJSON(w, http.StatusCreated, m)
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

type metricCheckRequest struct {
	CheckURL     bool   `json:"checkUrl"`
	CheckSession bool   `json:"checkSession"`
	CheckDisplay bool   `json:"checkDisplay"`
	Note         string `json:"note"`
}

// DeleteMetric handles DELETE /api/metrics/{id}.
func (h *Handler) DeleteMetric(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid metric id")
		return
	}
	if err := h.store.DeleteMetric(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "metric not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot delete metric")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": id})
}

// UpdateMetricChecks handles PATCH /api/metrics/{id}: the 3-step judgment flow.
func (h *Handler) UpdateMetricChecks(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid metric id")
		return
	}
	var req metricCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.store.UpdateMetricChecks(id, req.CheckURL, req.CheckSession, req.CheckDisplay, req.Note); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot update metric")
		return
	}
	m, err := h.store.GetMetric(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "metric not found")
		return
	}
	writeJSON(w, http.StatusOK, m)
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
