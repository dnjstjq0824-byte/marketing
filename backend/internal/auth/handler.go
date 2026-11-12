package auth

import (
	"encoding/json"
	"net/http"
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
