package member

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"somun/internal/store"
)

// Handler maps promo HTTP to the member service.
type Handler struct {
	svc *Service
}

// NewHandler builds the promo handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"error": msg})
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

// Signup handles POST /api/promo/signup and logs the member in.
func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	token, m, err := h.svc.Signup(req.Email, req.Password, req.Nickname)
	if err != nil {
		switch {
		case errors.Is(err, ErrDuplicate):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, ErrBadEmail), errors.Is(err, ErrWeakPassword), errors.Is(err, ErrBadNickname):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot sign up")
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "member": m})
}

// Login handles POST /api/promo/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	token, m, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrBadCredentials) {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot log in")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "member": m})
}

// Logout handles POST /api/promo/logout.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token, _ := bearerToken(r)
	h.svc.Logout(token)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Me handles GET /api/promo/me.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := MemberFrom(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "member login required")
		return
	}
	m, err := h.svc.Me(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// ListProducts handles GET /api/promo/products (public, no links inside).
func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	items := h.svc.ListProducts()
	writeJSON(w, http.StatusOK, map[string]any{"products": items})
}

// ProductDetail handles GET /api/promo/products/{id} (public, no link inside).
func (h *Handler) ProductDetail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	p, err := h.svc.ProductDetail(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

type applyRequest struct {
	Channels  string `json:"channels"`
	Plan      string `json:"plan"`
	FeeAgreed bool   `json:"feeAgreed"`
}

// Apply handles POST /api/promo/products/{id}/apply.
func (h *Handler) Apply(w http.ResponseWriter, r *http.Request) {
	id, ok := MemberFrom(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "member login required")
		return
	}
	pid, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	var req applyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	a, err := h.svc.Apply(id, pid, req.Channels, req.Plan, req.FeeAgreed)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoProduct):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrDuplicateApply):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, ErrRecruitClosed), errors.Is(err, ErrBadApply):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot apply")
		}
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

// MyApplications handles GET /api/promo/applications.
func (h *Handler) MyApplications(w http.ResponseWriter, r *http.Request) {
	id, ok := MemberFrom(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "member login required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applications": h.svc.MyApplications(id)})
}

// PrivateLink handles GET /api/promo/applications/{id}/private-link:
// the approved owner only, everyone else 403.
func (h *Handler) PrivateLink(w http.ResponseWriter, r *http.Request) {
	id, ok := MemberFrom(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "member login required")
		return
	}
	appID, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}
	url, err := h.svc.PrivateLink(id, appID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoApplication):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrForbidden):
			writeError(w, http.StatusForbidden, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot open link")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": url})
}

type productRequest struct {
	Name         string `json:"name"`
	Store        string `json:"store"`
	Price        int    `json:"price"`
	Photo        string `json:"photo"`
	Facts        string `json:"facts"`
	Points       string `json:"points"`
	BannedWords  string `json:"bannedWords"`
	Materials    string `json:"materials"`
	FeePerOrder  int    `json:"feePerOrder"`
	SettleTerms  string `json:"settleTerms"`
	PrivateURL   string `json:"privateUrl"`
	Category     string `json:"category"`
	RecruitUntil string `json:"recruitUntil"`
}

func (q *productRequest) input() ProductInput {
	return ProductInput{Name: q.Name, Store: q.Store, Price: q.Price, Photo: q.Photo,
		Facts: q.Facts, Points: q.Points, BannedWords: q.BannedWords, Materials: q.Materials,
		FeePerOrder: q.FeePerOrder, SettleTerms: q.SettleTerms, PrivateURL: q.PrivateURL,
		Category: q.Category, RecruitUntil: q.RecruitUntil}
}

// CreateProduct handles POST /api/promo/admin/products (operator).
func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req productRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	p, err := h.svc.CreateProduct(req.input())
	if err != nil {
		if errors.Is(err, ErrBadProduct) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot create product")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// UpdateProduct handles PATCH /api/promo/admin/products/{id} (operator).
func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	var req productRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	p, err := h.svc.UpdateProduct(id, req.input())
	if err != nil {
		switch {
		case errors.Is(err, ErrNoProduct):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrBadProduct):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot update product")
		}
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// ListProductsAdmin handles GET /api/promo/admin/products (operator).
func (h *Handler) ListProductsAdmin(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"products": h.svc.ListProductsAdmin()})
}

// Recruit handles POST /api/promo/admin/products/{id}/recruit (operator).
func (h *Handler) Recruit(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	p, err := h.svc.SetProductStatus(id, req.Status)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoProduct):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrBadProduct), errors.Is(err, ErrRecruitGate):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot set status")
		}
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// ListApplicationsAdmin handles GET /api/promo/admin/applications (operator).
func (h *Handler) ListApplicationsAdmin(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" && status != store.ApplyPending && status != store.ApplyApproved &&
		status != store.ApplyRejected {
		writeError(w, http.StatusBadRequest, "unknown status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applications": h.svc.ListApplicationsAdmin(status)})
}

// Decide handles POST /api/promo/admin/applications/{id}/decision (operator).
func (h *Handler) Decide(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}
	var req struct {
		Approve bool   `json:"approve"`
		Reason  string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	a, err := h.svc.Decide(id, req.Approve, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoApplication):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrBadDecision):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot decide")
		}
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// Overview handles GET /api/promo/admin/overview (operator).
func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.Overview())
}

// Work handles GET /api/promo/work: the member workspace.
func (h *Handler) Work(w http.ResponseWriter, r *http.Request) {
	id, ok := MemberFrom(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "member login required")
		return
	}
	writeJSON(w, http.StatusOK, h.svc.MyWork(id))
}

// RegisterPost handles POST /api/promo/applications/{id}/posts.
func (h *Handler) RegisterPost(w http.ResponseWriter, r *http.Request) {
	id, ok := MemberFrom(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "member login required")
		return
	}
	appID, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	p, err := h.svc.RegisterPost(id, appID, req.URL)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoApplication):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrForbidden):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, ErrBadURL):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot register post")
		}
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// ConfirmOrders handles POST /api/promo/admin/applications/{id}/confirm (operator).
func (h *Handler) ConfirmOrders(w http.ResponseWriter, r *http.Request) {
	appID, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}
	var req struct {
		Orders int `json:"orders"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.svc.ConfirmOrders(appID, req.Orders); err != nil {
		switch {
		case errors.Is(err, ErrNoApplication):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrBadDecision), errors.Is(err, ErrBadCount):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "cannot confirm orders")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
