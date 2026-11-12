package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"somun/internal/store"
)

// Handler serves operator auth endpoints.
type Handler struct {
	svc *Service
}

// NewHandler builds the auth handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login verifies credentials and issues a bearer token.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	token, user, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  map[string]any{"id": user.ID, "username": user.Username, "role": user.Role},
	})
}

// Me returns the current session user.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	token, err := BearerToken(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "operator login required")
		return
	}
	id, username, err := h.svc.Validate(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "username": username})
}

// Logout drops the current token.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if token, err := BearerToken(r); err == nil {
		h.svc.Logout(token)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ListUsers handles GET /api/users.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list users")
		return
	}
	views := make([]map[string]any, 0, len(users))
	for _, u := range users {
		views = append(views, map[string]any{"id": u.ID, "username": u.Username, "role": u.Role})
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": views})
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// CreateUser handles POST /api/users.
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	u, err := h.svc.CreateOperator(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, ErrWeakPassword) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusConflict, "cannot create user (duplicate name?)")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": map[string]any{"id": u.ID, "username": u.Username, "role": u.Role}})
}

// DeleteUser handles DELETE /api/users/{id}.
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	token, err := BearerToken(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "operator login required")
		return
	}
	callerID, _, err := h.svc.Validate(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}
	if err := h.svc.DeleteUserGuarded(callerID, id); err != nil {
		switch {
		case errors.Is(err, ErrDeleteSelf), errors.Is(err, ErrDeleteLast):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "user not found")
		default:
			writeError(w, http.StatusInternalServerError, "cannot delete user")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": id})
}

type passwordRequest struct {
	Current string `json:"current"`
	Next    string `json:"next"`
}

// ChangePassword handles PATCH /api/auth/password (self only).
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	token, err := BearerToken(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "operator login required")
		return
	}
	userID, username, err := h.svc.Validate(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}
	var req passwordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.svc.ChangeOwnPassword(userID, username, req.Current, req.Next); err != nil {
		switch {
		case errors.Is(err, ErrBadCredentials):
			writeError(w, http.StatusUnauthorized, "current password is wrong")
		case errors.Is(err, ErrWeakPassword):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot change password")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
