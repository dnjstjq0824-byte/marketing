package session

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// Handler serves slot/session endpoints.
type Handler struct {
	svc *Service
}

// NewHandler builds the session handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type registerRequest struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Owner    string `json:"owner"`
	Role     string `json:"role"`
	Business string `json:"business"`
	LoginID  string `json:"loginId"`
	Password string `json:"password"`
	Kind     string `json:"kind"`
}

// RegisterAccount handles POST /api/session/accounts.
func (h *Handler) RegisterAccount(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	// Validate the credential pair before creating anything: a half-supplied
	// pair must not leave a half-created account behind.
	if (req.LoginID != "") != (req.Password != "") {
		writeError(w, http.StatusBadRequest, "loginId and password must both be set or both empty")
		return
	}
	a, err := h.svc.RegisterAccount(strings.TrimSpace(req.Name), strings.TrimSpace(req.Platform),
		strings.TrimSpace(req.Owner), strings.TrimSpace(req.Role), strings.TrimSpace(req.Business))
	if err != nil {
		if errors.Is(err, ErrDuplicate) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.LoginID != "" || req.Password != "" {
		a, err = h.svc.SetCredentials(a.ID, req.LoginID, req.Password)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if strings.TrimSpace(req.Kind) != "" {
		kind := strings.TrimSpace(req.Kind)
		a, err = h.svc.SetKindFields(a.ID, &kind, nil)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusCreated, a)
}

type bulkRegisterRequest struct {
	Prefix   string `json:"prefix"`
	Platform string `json:"platform"`
	Owner    string `json:"owner"`
	Role     string `json:"role"`
	Business string `json:"business"`
	Count    int    `json:"count"`
	Kind     string `json:"kind"`
}

// RegisterBulk handles POST /api/session/accounts/bulk: creates Count
// accounts named <prefix>01..<prefix>NN for multi-account login queues.
func (h *Handler) RegisterBulk(w http.ResponseWriter, r *http.Request) {
	var req bulkRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	accounts, failures, err := h.svc.RegisterAccounts(
		strings.TrimSpace(req.Prefix), strings.TrimSpace(req.Platform),
		strings.TrimSpace(req.Owner), strings.TrimSpace(req.Role),
		strings.TrimSpace(req.Business), req.Count)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if accounts == nil {
		accounts = []*Account{}
	}
	if failures == nil {
		failures = []string{}
	}
	if kind := strings.TrimSpace(req.Kind); kind != "" {
		for i, a := range accounts {
			updated, err := h.svc.SetKindFields(a.ID, &kind, nil)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			accounts[i] = updated
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"accounts": accounts, "failures": failures})
}

// ListAccounts handles GET /api/session/accounts.
func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	items := h.svc.ListAccounts()
	if items == nil {
		items = []*Account{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": items})
}

type updateRequest struct {
	SessionState   *string `json:"sessionState"`
	Usage          *string `json:"usage"`
	Owner          *string `json:"owner"`
	Role           *string `json:"role"`
	Business       *string `json:"business"`
	PostingMode    *string `json:"postingMode"`
	DailyCap       *int    `json:"dailyCap"`
	LoginID        *string `json:"loginId"`
	Password       *string `json:"password"`
	Kind           *string `json:"kind"`
	PlatformUserID *string `json:"platformUserId"`
	PlanTier       *string `json:"planTier"`
	Capabilities   *string `json:"capabilities"`
}

// UpdateAccount handles PATCH /api/session/accounts/{id}: session state
// (with hold/resume side effects) and owner/role/business fields.
func (h *Handler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	// Same atomicity rule as registration: reject half-supplied credentials
	// before applying any other field.
	if req.LoginID != nil || req.Password != nil {
		lid, pw := "", ""
		if req.LoginID != nil {
			lid = *req.LoginID
		}
		if req.Password != nil {
			pw = *req.Password
		}
		if lid == "" || pw == "" {
			writeError(w, http.StatusBadRequest, "loginId and password must both be set or both empty")
			return
		}
	}
	var account *Account
	if req.SessionState != nil {
		usage := ""
		if req.Usage != nil {
			usage = strings.TrimSpace(*req.Usage)
		}
		account, err = h.svc.SetSessionState(id, strings.TrimSpace(*req.SessionState), usage)
		if err != nil {
			switch {
			case errors.Is(err, ErrNoAccount):
				writeError(w, http.StatusNotFound, err.Error())
			case errors.Is(err, ErrBadState):
				writeError(w, http.StatusBadRequest, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "cannot update session")
			}
			return
		}
	}
	if req.Owner != nil || req.Role != nil || req.Business != nil {
		trimmed := func(s *string) *string {
			if s == nil {
				return nil
			}
			v := strings.TrimSpace(*s)
			return &v
		}
		account, err = h.svc.UpdateAccountFields(id, trimmed(req.Owner), trimmed(req.Role), trimmed(req.Business))
		if err != nil {
			switch {
			case errors.Is(err, ErrNoAccount):
				writeError(w, http.StatusNotFound, err.Error())
			case errors.Is(err, ErrBadRole):
				writeError(w, http.StatusBadRequest, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "cannot update account")
			}
			return
		}
	}
	if req.PostingMode != nil {
		account, err = h.svc.SetPostingMode(id, strings.TrimSpace(*req.PostingMode))
		if err != nil {
			switch {
			case errors.Is(err, ErrNoAccount):
				writeError(w, http.StatusNotFound, err.Error())
			case errors.Is(err, ErrBadMode):
				writeError(w, http.StatusBadRequest, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "cannot set posting mode")
			}
			return
		}
	}
	if req.DailyCap != nil {
		account, err = h.svc.SetDailyCap(id, *req.DailyCap)
		if err != nil {
			switch {
			case errors.Is(err, ErrNoAccount):
				writeError(w, http.StatusNotFound, err.Error())
			case errors.Is(err, ErrBadDailyCap):
				writeError(w, http.StatusBadRequest, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "cannot set daily cap")
			}
			return
		}
	}
	if req.Kind != nil || req.PlatformUserID != nil {
		account, err = h.svc.SetKindFields(id, req.Kind, req.PlatformUserID)
		if err != nil {
			switch {
			case errors.Is(err, ErrNoAccount):
				writeError(w, http.StatusNotFound, err.Error())
			case errors.Is(err, ErrBadKind):
				writeError(w, http.StatusBadRequest, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "cannot set kind fields")
			}
			return
		}
	}
	if req.PlanTier != nil || req.Capabilities != nil {
		account, err = h.svc.SetPlanFields(id, req.PlanTier, req.Capabilities)
		if err != nil {
			switch {
			case errors.Is(err, ErrNoAccount):
				writeError(w, http.StatusNotFound, err.Error())
			case errors.Is(err, ErrBadCapabilities):
				writeError(w, http.StatusBadRequest, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "cannot set plan fields")
			}
			return
		}
	}
	if req.LoginID != nil || req.Password != nil {
		loginID, password := "", ""
		if req.LoginID != nil {
			loginID = *req.LoginID
		}
		if req.Password != nil {
			password = *req.Password
		}
		account, err = h.svc.SetCredentials(id, loginID, password)
		if err != nil {
			switch {
			case errors.Is(err, ErrNoAccount):
				writeError(w, http.StatusNotFound, err.Error())
			case errors.Is(err, ErrCredentials):
				writeError(w, http.StatusBadRequest, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "cannot save credentials")
			}
			return
		}
	}
	if account == nil {
		account, err = h.svc.GetAccount(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, account)
}

// Unregister handles DELETE /api/session/accounts/{id}.
func (h *Handler) Unregister(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}
	if err := h.svc.UnregisterAccount(id); err != nil {
		switch {
		case errors.Is(err, ErrNoAccount):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrHeldItems):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot unregister account")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": id})
}

// LoginStart handles POST /api/session/accounts/{id}/login-start: opens the
// headed login browser for the operator. Optional body {"realProfile": true}
// uses the operator's own Chrome profile (Chrome must be fully closed);
// {"incognito": true} opens a fresh incognito window instead (multi-account).
func (h *Handler) LoginStart(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}
	realProfile, incognito := false, false
	if r.Body != nil {
		var req struct {
			RealProfile bool `json:"realProfile"`
			Incognito   bool `json:"incognito"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			realProfile, incognito = req.RealProfile, req.Incognito
		}
	}
	pid, err := h.svc.StartLogin(id, realProfile, incognito)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoAccount):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrLoginBusy):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, ErrProfileBusy):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, ErrBadPlatform):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"started": true, "pid": pid})
}

// LoginFinish handles POST /api/session/accounts/{id}/login-finish: confirms
// the headed login, verifies the saved session, marks connected.
func (h *Handler) LoginFinish(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}
	a, err := h.svc.FinishLogin(id)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoAccount):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrNoLogin), errors.Is(err, ErrLoginFailed),
			errors.Is(err, ErrNotLoggedIn), errors.Is(err, ErrUnverifiable):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// NextLogin handles POST /api/session/login-queue/next {platform?}.
func (h *Handler) NextLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Platform string `json:"platform"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	account, err := h.svc.NextLogin(strings.TrimSpace(req.Platform))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load queue")
		return
	}
	if account == nil {
		writeJSON(w, http.StatusOK, map[string]any{"account": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"account": account})
}

// LoginCancel handles POST /api/session/accounts/{id}/login-cancel.
func (h *Handler) LoginCancel(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}
	h.svc.CancelLogin(id)
	writeJSON(w, http.StatusOK, map[string]any{"cancelled": true})
}

// ChatProbe handles POST /api/session/accounts/{id}/chat-probe (W15-4).
func (h *Handler) ChatProbe(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}
	res, err := h.svc.ChatProbe(id)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoAccount):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrBadPlatform):
			writeError(w, http.StatusBadRequest, "AI 플랫폼 계정이 아닙니다")
		default:
			writeError(w, http.StatusInternalServerError, "cannot probe chat")
		}
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ReloginQueue handles GET /api/session/relogin-queue.
func (h *Handler) ReloginQueue(w http.ResponseWriter, r *http.Request) {
	items := h.svc.ReloginQueue()
	if items == nil {
		items = []*ReloginItem{}
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

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
