package session

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
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
}

// RegisterAccount handles POST /api/session/accounts.
func (h *Handler) RegisterAccount(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
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
	writeJSON(w, http.StatusCreated, a)
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
	SessionState *string `json:"sessionState"`
	Usage        *string `json:"usage"`
	Owner        *string `json:"owner"`
	Role         *string `json:"role"`
	Business     *string `json:"business"`
	PostingMode  *string `json:"postingMode"`
	LoginID      *string `json:"loginId"`
	Password     *string `json:"password"`
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

// CheckNow handles POST /api/session/check (light inspection skeleton).
func (h *Handler) CheckNow(w http.ResponseWriter, r *http.Request) {
	n := h.svc.CheckNow()
	writeJSON(w, http.StatusOK, map[string]any{"checked": n})
}

// Wake handles POST /api/session/accounts/{id}/wake (assigned sessions only).
func (h *Handler) Wake(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}
	a, err := h.svc.Wake(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// PublishPlan handles GET /api/publish/plan.
func (h *Handler) PublishPlan(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": h.svc.PublishPlan(time.Now())})
}

// ReloginQueue handles GET /api/session/relogin-queue.
func (h *Handler) ReloginQueue(w http.ResponseWriter, r *http.Request) {
	items := h.svc.ReloginQueue()
	if items == nil {
		items = []*ReloginItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ListSlots handles GET /api/session/slots.
func (h *Handler) ListSlots(w http.ResponseWriter, r *http.Request) {
	items := h.svc.ListSlots()
	if items == nil {
		items = []*Slot{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"slots": items})
}

type slotRequest struct {
	Label     string `json:"label"`
	AgentName string `json:"agentName"`
}

// AddSlot handles POST /api/session/slots.
func (h *Handler) AddSlot(w http.ResponseWriter, r *http.Request) {
	var req slotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	sl, err := h.svc.AddSlot(strings.TrimSpace(req.Label), strings.TrimSpace(req.AgentName))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sl)
}

// UpdateSlot handles PATCH /api/session/slots/{id}.
func (h *Handler) UpdateSlot(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid slot id")
		return
	}
	var req struct {
		Label     *string `json:"label"`
		AgentName *string `json:"agentName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	trimmed := func(s *string) *string {
		if s == nil {
			return nil
		}
		v := strings.TrimSpace(*s)
		return &v
	}
	sl, err := h.svc.UpdateSlot(id, trimmed(req.Label), trimmed(req.AgentName))
	if err != nil {
		switch {
		case errors.Is(err, ErrNoSlot):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, sl)
}

// DeleteSlot handles DELETE /api/session/slots/{id}.
func (h *Handler) DeleteSlot(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid slot id")
		return
	}
	if err := h.svc.DeleteSlot(id); err != nil {
		if errors.Is(err, ErrNoSlot) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot delete slot")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": id})
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
