package chat

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// Handler serves chat endpoints for console and workers.
type Handler struct {
	svc *Service
}

// NewHandler builds the chat handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type sendRequest struct {
	AccountID   int64    `json:"accountId"`
	Message     string   `json:"message"`
	Thread      string   `json:"thread"`
	Attachments []string `json:"attachments"`
}

// Send handles POST /api/chat/send and POST /api/worker/chat/send.
// Fast runs answer 200 with the reply; slow runs answer 202 with a pending
// handle the caller polls via Pending.
func (h *Handler) Send(w http.ResponseWriter, r *http.Request) {
	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	res, err := h.svc.Send(req.AccountID, req.Message, req.Thread, req.Attachments...)
	if err != nil {
		writeSendError(w, err)
		return
	}
	if res.Pending {
		writeJSON(w, http.StatusAccepted, map[string]any{"pending": true, "afterId": res.UserMessage})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reply": res.Reply})
}

// Pending handles GET /api/chat/pending?afterId= and the worker twin.
// ResetThread handles POST /api/chat/threads/reset: drops the saved thread
// URLs so the next message starts a fresh conversation (S5-C 새 대화).
func (h *Handler) ResetThread(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID int64  `json:"accountId"`
		Thread    string `json:"thread"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.svc.ResetThread(req.AccountID, req.Thread); err != nil {
		switch {
		case errors.Is(err, ErrNoAccount):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrBadThread):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot reset thread")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) Pending(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("afterId"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "afterId is required")
		return
	}
	reply, done, failed, message := h.svc.Pending(id)
	writeJSON(w, http.StatusOK, map[string]any{
		"done": done, "failed": failed, "reply": reply, "message": message,
	})
}

func writeSendError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNoAccount):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrEmptyMessage), errors.Is(err, ErrMessageTooBig), errors.Is(err, ErrBadThread), errors.Is(err, ErrBadAttachment):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNotConnected):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrUnavailable), errors.Is(err, ErrChatNoSession):
		writeError(w, http.StatusNotImplemented, err.Error())
	default:
		writeError(w, http.StatusBadGateway, err.Error())
	}
}

// History handles GET /api/chat/history?accountId=&limit=.
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("accountId"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "accountId is required")
		return
	}
	limit := 30
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = n
	}
	items, err := h.svc.History(id, limit)
	if err != nil {
		if errors.Is(err, ErrNoAccount) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot list history")
		return
	}
	if items == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
