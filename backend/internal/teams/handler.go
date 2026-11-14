package teams

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"somun/internal/store"
)

// Handler serves the teams API (S7/S7-D).
type Handler struct {
	svc   *Service
	store *store.Store
}

// NewHandler builds the teams handler.
func NewHandler(svc *Service, st *store.Store) *Handler {
	return &Handler{svc: svc, store: st}
}

// List handles GET /api/teams.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	cards, err := h.svc.ListCards()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list teams")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": cards})
}

// Create handles POST /api/teams.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Businesses  []string `json:"businesses"`
		RoleMode    string   `json:"roleMode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	t, err := h.svc.Create(req.Name, req.Description, req.Businesses, req.RoleMode)
	if err != nil {
		h.createErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handler) createErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrDupName), errors.Is(err, ErrBusinessTaken):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrNoName), errors.Is(err, ErrBadRoleMode):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "cannot create team")
	}
}

// Detail handles GET /api/teams/{id} → {team, brand, members, works, channels, stats}.
func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	d, err := h.svc.Detail(id)
	if err != nil {
		if errors.Is(err, ErrNoTeam) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot load team")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// Update handles PATCH /api/teams/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	var req struct {
		Name           *string  `json:"name"`
		Description    *string  `json:"description"`
		Businesses     []string `json:"businesses"`
		RoleMode       *string  `json:"roleMode"`
		MaxConcurrency *int     `json:"maxConcurrency"`
		Enabled        *bool    `json:"enabled"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	var keys map[string]any
	_ = json.Unmarshal(raw, &keys)
	_, hasBusinesses := keys["businesses"]
	t, err := h.svc.Update(id, req.Name, req.Description, req.Businesses, hasBusinesses,
		req.RoleMode, req.MaxConcurrency, req.Enabled)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoTeam):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrDupName), errors.Is(err, ErrBusinessTaken):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, ErrNoName), errors.Is(err, ErrBadRoleMode):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot update team")
		}
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// Delete handles DELETE /api/teams/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	moved, running, err := h.svc.Delete(id)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoTeam):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrDefaultProtect):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot delete team")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": id, "moved": moved, "runningWorks": running})
}

// AddMember handles POST /api/teams/{id}/members {accountId}.
func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	id, accountID, ok := h.memberArgs(w, r)
	if !ok {
		return
	}
	if err := h.svc.AddMember(id, accountID); err != nil {
		h.memberErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

// RemoveMember handles DELETE /api/teams/{id}/members/{accountId}.
func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	id, accountID, ok := h.memberPath(w, r)
	if !ok {
		return
	}
	if err := h.svc.RemoveMember(id, accountID); err != nil {
		h.memberErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// SetLeader handles POST /api/teams/{id}/leader {accountId|null}.
func (h *Handler) SetLeader(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	var req struct {
		AccountID *int64 `json:"accountId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	t, err := h.svc.SetLeader(id, req.AccountID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoTeam):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrNotMember):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot set leader")
		}
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// SetMemberRole handles POST /api/teams/{id}/members/{accountId}/roles {roles[]}.
func (h *Handler) SetMemberRole(w http.ResponseWriter, r *http.Request) {
	id, accountID, ok := h.memberPath(w, r)
	if !ok {
		return
	}
	var req struct {
		Roles []string `json:"roles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.svc.SetMemberRole(id, accountID, req.Roles); err != nil {
		h.memberErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// AddChannel handles POST /api/teams/{id}/channels {accountId}.
func (h *Handler) AddChannel(w http.ResponseWriter, r *http.Request) {
	id, accountID, ok := h.memberArgs(w, r)
	if !ok {
		return
	}
	if err := h.svc.AddChannel(id, accountID); err != nil {
		h.memberErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

// RemoveChannel handles DELETE /api/teams/{id}/channels/{accountId}.
func (h *Handler) RemoveChannel(w http.ResponseWriter, r *http.Request) {
	id, accountID, ok := h.memberPath(w, r)
	if !ok {
		return
	}
	if err := h.svc.RemoveChannel(id, accountID); err != nil {
		h.memberErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PutMembers handles PUT /api/teams/{id}/members {members:[{accountId,roles}]}.
// Full replacement: listed accounts move in, everyone else leaves.
func (h *Handler) PutMembers(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	var req struct {
		Members []struct {
			AccountID int64    `json:"accountId"`
			Roles     []string `json:"roles"`
		} `json:"members"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	list := make([]MemberAssign, 0, len(req.Members))
	for _, m := range req.Members {
		if m.AccountID <= 0 {
			writeError(w, http.StatusBadRequest, "accountId is required")
			return
		}
		list = append(list, MemberAssign{AccountID: m.AccountID, Roles: m.Roles})
	}
	if err := h.svc.SetMembers(id, list); err != nil {
		h.memberErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PutChannels handles PUT /api/teams/{id}/channels {channels:[accountId]}.
// Full replacement of this team's channel list.
func (h *Handler) PutChannels(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	var req struct {
		Channels []int64 `json:"channels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.svc.SetChannels(id, req.Channels); err != nil {
		h.memberErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PutBrand handles PUT /api/teams/{id}/brand {intro,tone,bannedWords,buyLink}.
func (h *Handler) PutBrand(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	var req struct {
		Intro       string `json:"intro"`
		Tone        string `json:"tone"`
		BannedWords string `json:"bannedWords"`
		BuyLink     string `json:"buyLink"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := h.svc.UpsertBrand(id, req.Intro, req.Tone, req.BannedWords, req.BuyLink)
	if err != nil {
		if errors.Is(err, ErrNoTeam) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot save brand")
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (h *Handler) memberArgs(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid team id")
		return 0, 0, false
	}
	var req struct {
		AccountID int64 `json:"accountId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AccountID <= 0 {
		writeError(w, http.StatusBadRequest, "accountId is required")
		return 0, 0, false
	}
	return id, req.AccountID, true
}

func (h *Handler) memberPath(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid team id")
		return 0, 0, false
	}
	accountID, err := strconv.ParseInt(r.PathValue("accountId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return 0, 0, false
	}
	return id, accountID, true
}

func (h *Handler) memberErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNoTeam), errors.Is(err, ErrNoAccount), errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrNotMember), errors.Is(err, ErrBadRole),
		errors.Is(err, ErrNotAgent), errors.Is(err, ErrNotChannel), errors.Is(err, ErrDupMember):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "cannot update team")
	}
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
