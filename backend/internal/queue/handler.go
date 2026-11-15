package queue

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"somun/internal/agent"
	"somun/internal/store"
)

// Handler serves the worker API under /api/worker.
type Handler struct {
	svc         *Service
	agents      *agent.Service
	workerToken string
	allowShared bool // dev only; production serves agent tokens alone
	// Production, when true, refuses fake- workers (W9). Set by the router.
	Production bool
}

// FakeWorkerPrefix marks test-loop workers. worker.py --fake enforces it.
const FakeWorkerPrefix = "fake-"

func isFakeName(name string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(name)), FakeWorkerPrefix)
}

// fakeBlocked reports whether this request comes from a fake worker that
// production must refuse. Agent identity wins when present.
func (h *Handler) fakeBlocked(r *http.Request) bool {
	if !h.Production {
		return false
	}
	if a, ok := agent.AgentFrom(r.Context()); ok && a != nil {
		return isFakeName(a.Label)
	}
	return isFakeName(workerName(r))
}

// NewHandler builds the worker handler.
func NewHandler(svc *Service, agents *agent.Service, workerToken string, allowShared bool) *Handler {
	return &Handler{svc: svc, agents: agents, workerToken: workerToken, allowShared: allowShared}
}

// Auth resolves agent tokens first (identity from the token, D23), then the
// shared dev token. Paused agents and production shared use are refused.
func (h *Handler) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := r.Header.Get("Authorization")
		parts := strings.SplitN(hdr, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || parts[1] == "" {
			writeError(w, http.StatusUnauthorized, "worker token required")
			return
		}
		token := parts[1]
		if a, err := h.agents.Authenticate(token); err == nil {
			next.ServeHTTP(w, r.WithContext(agent.WithAgent(r.Context(), a)))
			return
		} else if errors.Is(err, agent.ErrAgentPaused) {
			writeError(w, http.StatusForbidden, "agent is paused")
			return
		}
		if h.allowShared && token == h.workerToken {
			next.ServeHTTP(w, r.WithContext(agent.WithShared(r.Context())))
			return
		}
		writeError(w, http.StatusUnauthorized, "invalid worker token")
	})
}

func workerName(r *http.Request) string {
	if n := strings.TrimSpace(r.Header.Get("X-Worker-Name")); n != "" {
		return n
	}
	return DefaultWorkerName
}

// Next handles GET /api/worker/jobs/next?type=&wait=. Agents may omit the
// type (whole lane) and long-poll; the shared token keeps type required.
func (h *Handler) Next(w http.ResponseWriter, r *http.Request) {
	if h.fakeBlocked(r) {
		writeError(w, http.StatusForbidden, "fake workers are refused in production")
		return
	}
	a, _ := agent.AgentFrom(r.Context())
	wait, err := parseWait(r.URL.Query().Get("wait"))
	if err != nil || wait < 0 {
		writeError(w, http.StatusBadRequest, "wait must be a non-negative integer")
		return
	}
	if a != nil {
		jobType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
		if jobType != "" {
			if err := ValidateJobType(jobType); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		job, err := h.agents.Claim(a, jobType, wait)
		if err != nil {
			if errors.Is(err, agent.ErrLaneMismatch) {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "cannot claim job")
			return
		}
		writeClaimed(w, job)
		return
	}
	jobType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if err := ValidateJobType(jobType); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	job, err := h.svc.Next(jobType, workerName(r))
	if err == nil && job == nil && wait > 0 {
		h.agents.Wait(wait)
		job, err = h.svc.Next(jobType, workerName(r))
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot claim job")
		return
	}
	writeClaimed(w, job)
}

func parseWait(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}

func writeClaimed(w http.ResponseWriter, job *store.Job) {
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
// operator auth, for failure review and worker-install cleanup.
func (h *Handler) ListForOperator(w http.ResponseWriter, r *http.Request) {
	h.listJobs(w)
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

// BootCheck handles GET /api/worker/boot-check?accountId=. Workers call it
// at boot; ok=false means the account spans B+C and must not start (W6).
func (h *Handler) BootCheck(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("accountId")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "accountId is required")
		return
	}
	ok, lanes, err := h.svc.BootCheck(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot check boot")
		return
	}
	out := map[string]any{"ok": ok, "lanes": lanes}
	if !ok {
		out["reason"] = "account spans lanes B+C: one account cannot write and review"
	}
	writeJSON(w, http.StatusOK, out)
}

// Result handles POST /api/worker/jobs/{id}/result. Not idempotent.
func (h *Handler) Result(w http.ResponseWriter, r *http.Request) {
	if h.fakeBlocked(r) {
		writeError(w, http.StatusForbidden, "fake workers are refused in production")
		return
	}
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
	if a, _ := agent.AgentFrom(r.Context()); a != nil {
		if job.Status != StatusClaimed || job.AgentID != a.ID {
			writeError(w, http.StatusConflict, "job is not claimed by this agent")
			return
		}
	}
	switch job.Type {
	case JobResearch:
		var rres ResearchResult
		if err := json.NewDecoder(r.Body).Decode(&rres); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body: all research fields must be strings")
			return
		}
		done, err := h.svc.SubmitResearch(id, rres)
		if err != nil {
			h.writeSubmitError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"job": toView(done)})
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
	case JobPublish:
		var res PublishResult
		if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		done, err := h.svc.SubmitPublish(id, res)
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
		errors.Is(err, ErrEmptyDecisions), errors.Is(err, ErrBadDecision),
		errors.Is(err, ErrNoURL), errors.Is(err, ErrNotQueued):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrJobNotAnalyze), errors.Is(err, ErrJobNotStrategy), errors.Is(err, ErrJobNotGenerate),
		errors.Is(err, ErrJobNotReview), errors.Is(err, ErrJobNotPublish):
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
	if h.fakeBlocked(r) {
		writeError(w, http.StatusForbidden, "fake workers are refused in production")
		return
	}
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
	if a, _ := agent.AgentFrom(r.Context()); a != nil {
		cur, err := h.svc.store.GetJob(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "job not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "cannot load job")
			return
		}
		if cur.Status != StatusClaimed || cur.AgentID != a.ID {
			writeError(w, http.StatusConflict, "job is not claimed by this agent")
			return
		}
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
