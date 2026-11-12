package queue

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"somun/internal/store"
)

// Handler serves the worker API under /api/worker.
type Handler struct {
	svc         *Service
	workerToken string
}

// NewHandler builds the worker handler.
func NewHandler(svc *Service, workerToken string) *Handler {
	return &Handler{svc: svc, workerToken: workerToken}
}

// Auth rejects requests without the worker bearer token.
func (h *Handler) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := r.Header.Get("Authorization")
		parts := strings.SplitN(hdr, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || parts[1] == "" {
			writeError(w, http.StatusUnauthorized, "worker token required")
			return
		}
		if parts[1] != h.workerToken {
			writeError(w, http.StatusUnauthorized, "invalid worker token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func workerName(r *http.Request) string {
	if n := strings.TrimSpace(r.Header.Get("X-Worker-Name")); n != "" {
		return n
	}
	return DefaultWorkerName
}

// Next handles GET /api/worker/jobs/next?type=analyze|generate.
func (h *Handler) Next(w http.ResponseWriter, r *http.Request) {
	jobType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if err := ValidateJobType(jobType); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	job, err := h.svc.Next(jobType, workerName(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot claim job")
		return
	}
	if job == nil {
		writeJSON(w, http.StatusOK, map[string]any{"job": nil, "message": EmptyQueueMessage})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": toView(job)})
}

// List handles GET /api/worker/jobs (newest 100).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	h.listJobs(w)
}

// ListForOperator handles GET /api/jobs: the same newest-100 list behind
// operator auth, so a human can review failed jobs on the queue screen.
func (h *Handler) ListForOperator(w http.ResponseWriter, r *http.Request) {
	h.listJobs(w)
}

type cancelRequest struct {
	Reason string `json:"reason"`
}

// CancelForOperator handles POST /api/jobs/{id}/cancel: a pending/claimed job
// is marked failed like a worker failure (never requeues). Finished jobs 409.
func (h *Handler) CancelForOperator(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	var req cancelRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "cancelled by operator"
	}
	job, err := h.svc.store.SetJobFailed(id, reason)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "job not found")
		case errors.Is(err, store.ErrAlreadyDone):
			writeError(w, http.StatusConflict, "job already finished")
		default:
			writeError(w, http.StatusInternalServerError, "cannot cancel job")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": toView(job)})
}

// DeleteForOperator handles DELETE /api/jobs/{id}: purge one job row.
func (h *Handler) DeleteForOperator(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	if err := h.svc.store.DeleteJob(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot delete job")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": id})
}

func (h *Handler) listJobs(w http.ResponseWriter) {
	jobs, err := h.svc.Recent()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list jobs")
		return
	}
	views := make([]*JobView, 0, len(jobs))
	for _, j := range jobs {
		views = append(views, toView(j))
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": views})
}

// Result handles POST /api/worker/jobs/{id}/result. Not idempotent.
func (h *Handler) Result(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	job, err := h.svc.store.GetJob(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot load job")
		return
	}
	switch job.Type {
	case JobAnalyze:
		var res AnalyzeResult
		if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body: all analyze fields must be strings")
			return
		}
		done, err := h.svc.SubmitAnalyze(id, res)
		if err != nil {
			h.writeSubmitError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"job": toView(done)})
	case JobStrategy:
		var res StrategyResult
		if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body: all strategy fields must be strings")
			return
		}
		done, err := h.svc.SubmitStrategy(id, res)
		if err != nil {
			h.writeSubmitError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"job": toView(done)})
	case JobGenerate:
		var res GenerateResult
		if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		done, kept, err := h.svc.SubmitGenerate(id, res)
		if err != nil {
			h.writeSubmitError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"job": toView(done), "drafts": kept})
	case JobReview:
		var res ReviewResult
		if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		done, err := h.svc.SubmitReview(id, res)
		if err != nil {
			h.writeSubmitError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"job": toView(done)})
	default:
		writeError(w, http.StatusBadRequest, "unknown job type")
	}
}

func (h *Handler) writeSubmitError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrAlreadyDone):
		writeError(w, http.StatusBadRequest, "result already registered for this job")
	case errors.Is(err, store.ErrNotClaimed):
		writeError(w, http.StatusConflict, "job must be claimed before result registration")
	case errors.Is(err, ErrEmptyContents), errors.Is(err, ErrNoValidItems), errors.Is(err, ErrEmptyAngle),
		errors.Is(err, ErrEmptyDecisions), errors.Is(err, ErrBadDecision):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrJobNotAnalyze), errors.Is(err, ErrJobNotStrategy), errors.Is(err, ErrJobNotGenerate),
		errors.Is(err, ErrJobNotReview):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "job not found")
	default:
		writeError(w, http.StatusInternalServerError, "cannot register result")
	}
}

type failRequest struct {
	Error string `json:"error"`
}

// Fail handles POST /api/worker/jobs/{id}/fail.
func (h *Handler) Fail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	var req failRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	job, err := h.svc.Fail(id, req.Error)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrAlreadyDone):
			writeError(w, http.StatusBadRequest, "job already finished")
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "job not found")
		default:
			if err.Error() == "error reason is required" {
				writeError(w, http.StatusBadRequest, err.Error())
			} else {
				writeError(w, http.StatusInternalServerError, "cannot fail job")
			}
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": toView(job)})
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
