package member

import (
	"errors"
	"strings"
	"time"

	"somun/internal/store"
)

var (
	ErrNoProduct      = errors.New("product not found")
	ErrBadProduct     = errors.New("product name and fee are required")
	ErrRecruitGate    = errors.New("recruiting needs a per-order fee and the private link")
	ErrRecruitClosed  = errors.New("recruiting is closed for this product")
	ErrNoApplication  = errors.New("application not found")
	ErrBadApply       = errors.New("channels, plan, and fee agreement are required")
	ErrDuplicateApply = errors.New("already applied for this product")
	ErrBadDecision    = errors.New("only pending applications decide, rejections need a reason")
	ErrForbidden      = errors.New("available after approval")
	ErrBadURL         = errors.New("posted URL must start with http(s)://")
	ErrBadCount       = errors.New("confirmed orders must be 0 or more")
)

// ProductView is the public shape: the private link never appears here.
type ProductView struct {
	ID           int64  `json:"id"`
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
	Category     string `json:"category"`
	RecruitUntil string `json:"recruitUntil"`
	Applicants   int    `json:"applicants"`
	Status       string `json:"status"`
}

func toProductView(p *store.ConsignmentProduct) *ProductView {
	return &ProductView{ID: p.ID, Name: p.Name, Store: p.Store, Price: p.Price,
		Photo: p.Photo, Facts: p.Facts, Points: p.Points, BannedWords: p.BannedWords,
		Materials: p.Materials, FeePerOrder: p.FeePerOrder, SettleTerms: p.SettleTerms,
		Category: p.Category, RecruitUntil: p.RecruitUntil, Status: p.Status}
}

// ProductAdminView adds the masked link state for operators.
type ProductAdminView struct {
	ProductView
	LinkMasked string `json:"linkMasked"`
}

func (s *Service) maskLink(enc string) string {
	if enc == "" {
		return ""
	}
	plain, err := s.open(enc)
	if err != nil || plain == "" {
		return "저장됨"
	}
	if len(plain) > 25 {
		return plain[:25] + "•••"
	}
	return plain + "•••"
}

// ListProducts returns public listings, recruiting and closed alike,
// each with its applicant count.
func (s *Service) ListProducts() []*ProductView {
	rows, err := s.store.ListConsignmentProducts("")
	if err != nil {
		return []*ProductView{}
	}
	counts, _ := s.store.ApplicationCounts()
	out := make([]*ProductView, 0, len(rows))
	for _, p := range rows {
		v := toProductView(p)
		v.Applicants = counts[p.ID]
		out = append(out, v)
	}
	return out
}

// ProductDetail returns one listing with its applicant count.
// The link stays out by construction.
func (s *Service) ProductDetail(id int64) (*ProductView, error) {
	p, err := s.store.GetConsignmentProduct(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoProduct
		}
		return nil, err
	}
	v := toProductView(p)
	if counts, cerr := s.store.ApplicationCounts(); cerr == nil {
		v.Applicants = counts[p.ID]
	}
	return v, nil
}

// ApplicationView enriches one request with its product facts.
type ApplicationView struct {
	ID        int64  `json:"id"`
	ProductID int64  `json:"productId"`
	Product   string `json:"product"`
	FeeAgreed int    `json:"feeAgreed"`
	Channels  string `json:"channels"`
	Plan      string `json:"plan"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"createdAt"`
}

// Apply files one promotion request. Closed products and repeats refuse;
// the fee snapshot freezes what the applicant agreed to.
func (s *Service) Apply(memberID, productID int64, channels, plan string, feeAgreed bool) (*ApplicationView, error) {
	p, err := s.store.GetConsignmentProduct(productID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoProduct
		}
		return nil, err
	}
	if p.Status != store.ConsignRecruiting {
		return nil, ErrRecruitClosed
	}
	if strings.TrimSpace(channels) == "" || strings.TrimSpace(plan) == "" || !feeAgreed {
		return nil, ErrBadApply
	}
	a, err := s.store.CreateApplication(&store.ConsignmentApplication{
		ProductID: productID, MemberID: memberID,
		Channels: strings.TrimSpace(channels), Plan: strings.TrimSpace(plan),
		FeeAgreed: p.FeePerOrder,
	})
	if err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil, ErrDuplicateApply
		}
		return nil, err
	}
	return s.applicationView(a)
}

// MyApplications lists one member's requests with product facts.
func (s *Service) MyApplications(memberID int64) []*ApplicationView {
	rows, err := s.store.ListMemberApplications(memberID)
	if err != nil {
		return []*ApplicationView{}
	}
	out := make([]*ApplicationView, 0, len(rows))
	for _, a := range rows {
		v, err := s.applicationView(a)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}

func (s *Service) applicationView(a *store.ConsignmentApplication) (*ApplicationView, error) {
	p, err := s.store.GetConsignmentProduct(a.ProductID)
	if err != nil {
		return nil, err
	}
	return &ApplicationView{ID: a.ID, ProductID: a.ProductID, Product: p.Name,
		FeeAgreed: a.FeeAgreed, Channels: a.Channels, Plan: a.Plan,
		Status: a.Status, Reason: a.Reason, CreatedAt: a.CreatedAt}, nil
}

// PrivateLink opens the sealed URL for the approved owner only.
func (s *Service) PrivateLink(memberID, appID int64) (string, error) {
	a, err := s.store.GetApplication(appID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", ErrNoApplication
		}
		return "", err
	}
	if a.MemberID != memberID || a.Status != store.ApplyApproved {
		return "", ErrForbidden
	}
	p, err := s.store.GetConsignmentProduct(a.ProductID)
	if err != nil {
		return "", err
	}
	return s.open(p.PrivateURLEnc)
}

// ProductInput carries admin product writes; PrivateURL is plaintext here
// and sealed before storage.
func cleanProductInput(in ProductInput) (ProductInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Store = strings.TrimSpace(in.Store)
	in.Photo = strings.TrimSpace(in.Photo)
	in.PrivateURL = strings.TrimSpace(in.PrivateURL)
	in.Category = strings.TrimSpace(in.Category)
	in.RecruitUntil = strings.TrimSpace(in.RecruitUntil)
	if in.Name == "" {
		return in, ErrBadProduct
	}
	if len(in.Category) > 20 {
		return in, ErrBadProduct
	}
	if in.RecruitUntil != "" {
		if _, err := time.Parse("2006-01-02", in.RecruitUntil); err != nil {
			return in, ErrBadProduct
		}
	}
	return in, nil
}

type ProductInput struct {
	Name         string
	Store        string
	Price        int
	Photo        string
	Facts        string
	Points       string
	BannedWords  string
	Materials    string
	FeePerOrder  int
	SettleTerms  string
	PrivateURL   string
	Category     string
	RecruitUntil string
	Status       string
}

// CreateProduct registers one listing. New rows start closed; recruiting
// opens only through SetProductStatus once fee and link exist.
func (s *Service) CreateProduct(in ProductInput) (*ProductAdminView, error) {
	in, err := cleanProductInput(in)
	if err != nil {
		return nil, err
	}
	enc, err := s.seal(in.PrivateURL)
	if err != nil {
		return nil, err
	}
	p, err := s.store.CreateConsignmentProduct(&store.ConsignmentProduct{
		Name: in.Name, Store: in.Store, Price: in.Price, Photo: in.Photo,
		Facts: in.Facts, Points: in.Points, BannedWords: in.BannedWords,
		Materials: in.Materials, FeePerOrder: in.FeePerOrder,
		SettleTerms: in.SettleTerms, PrivateURLEnc: enc,
		Category: in.Category, RecruitUntil: in.RecruitUntil, Status: store.ConsignClosed,
	})
	if err != nil {
		return nil, err
	}
	return s.productAdminView(p), nil
}

// UpdateProduct rewrites editable fields; status moves via SetProductStatus.
func (s *Service) UpdateProduct(id int64, in ProductInput) (*ProductAdminView, error) {
	p, err := s.store.GetConsignmentProduct(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoProduct
		}
		return nil, err
	}
	in, err = cleanProductInput(in)
	if err != nil {
		return nil, err
	}
	enc := p.PrivateURLEnc
	if in.PrivateURL != "" {
		enc, err = s.seal(in.PrivateURL)
		if err != nil {
			return nil, err
		}
	}
	p.Name = in.Name
	p.Store = in.Store
	p.Price = in.Price
	p.Photo = in.Photo
	p.Facts = in.Facts
	p.Points = in.Points
	p.BannedWords = in.BannedWords
	p.Materials = in.Materials
	p.FeePerOrder = in.FeePerOrder
	p.SettleTerms = in.SettleTerms
	p.Category = in.Category
	p.RecruitUntil = in.RecruitUntil
	p.PrivateURLEnc = enc
	if err := s.store.UpdateConsignmentProduct(p); err != nil {
		return nil, err
	}
	return s.productAdminView(p), nil
}

// SetProductStatus opens or closes recruiting. Opening requires the
// per-order fee and the private link: applicants must see pay and get
// the link on approval, with no gaps.
func (s *Service) SetProductStatus(id int64, status string) (*ProductAdminView, error) {
	if status != store.ConsignRecruiting && status != store.ConsignClosed {
		return nil, ErrBadProduct
	}
	p, err := s.store.GetConsignmentProduct(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoProduct
		}
		return nil, err
	}
	if status == store.ConsignRecruiting && (p.FeePerOrder <= 0 || p.PrivateURLEnc == "") {
		return nil, ErrRecruitGate
	}
	p.Status = status
	if err := s.store.UpdateConsignmentProduct(p); err != nil {
		return nil, err
	}
	return s.productAdminView(p), nil
}

func (s *Service) productAdminView(p *store.ConsignmentProduct) *ProductAdminView {
	return &ProductAdminView{ProductView: *toProductView(p), LinkMasked: s.maskLink(p.PrivateURLEnc)}
}

// ListProductsAdmin returns every listing with masked link states.
func (s *Service) ListProductsAdmin() []*ProductAdminView {
	rows, err := s.store.ListConsignmentProducts("")
	if err != nil {
		return []*ProductAdminView{}
	}
	out := make([]*ProductAdminView, 0, len(rows))
	for _, p := range rows {
		out = append(out, s.productAdminView(p))
	}
	return out
}

// ApplicationAdminView enriches one request with member and product facts.
type ApplicationAdminView struct {
	ApplicationView
	MemberEmail     string `json:"memberEmail"`
	MemberNickname  string `json:"memberNickname"`
	ConfirmedOrders int    `json:"confirmedOrders"`
}

// ListApplicationsAdmin returns requests, optionally one status.
func (s *Service) ListApplicationsAdmin(status string) []*ApplicationAdminView {
	rows, err := s.store.ListApplications(status)
	if err != nil {
		return []*ApplicationAdminView{}
	}
	out := make([]*ApplicationAdminView, 0, len(rows))
	for _, a := range rows {
		v, err := s.applicationView(a)
		if err != nil {
			continue
		}
		av := &ApplicationAdminView{ApplicationView: *v, ConfirmedOrders: a.ConfirmedOrders}
		if m, err := s.store.GetMember(a.MemberID); err == nil {
			av.MemberEmail, av.MemberNickname = m.Email, m.Nickname
		}
		out = append(out, av)
	}
	return out
}

// Decide approves or rejects one pending request. Rejections need a reason;
// approvals open the private link immediately.
func (s *Service) Decide(appID int64, approve bool, reason string) (*ApplicationAdminView, error) {
	a, err := s.store.GetApplication(appID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoApplication
		}
		return nil, err
	}
	if a.Status != store.ApplyPending {
		return nil, ErrBadDecision
	}
	reason = strings.TrimSpace(reason)
	if !approve && reason == "" {
		return nil, ErrBadDecision
	}
	status := store.ApplyApproved
	if !approve {
		status = store.ApplyRejected
	}
	if err := s.store.DecideApplication(appID, status, reason); err != nil {
		return nil, err
	}
	return s.applicationAdminView(appID)
}

func (s *Service) applicationAdminView(appID int64) (*ApplicationAdminView, error) {
	a, err := s.store.GetApplication(appID)
	if err != nil {
		return nil, err
	}
	v, err := s.applicationView(a)
	if err != nil {
		return nil, err
	}
	av := &ApplicationAdminView{ApplicationView: *v, ConfirmedOrders: a.ConfirmedOrders}
	if m, err := s.store.GetMember(a.MemberID); err == nil {
		av.MemberEmail, av.MemberNickname = m.Email, m.Nickname
	}
	return av, nil
}

// Overview counts the admin dashboard: products, waiting requests,
// approved promoters, and the payable sum over confirmed orders.
func (s *Service) Overview() map[string]any {
	products, _ := s.store.CountConsignmentProducts("")
	pending, _ := s.store.CountApplicationsByStatus(store.ApplyPending)
	promoters, _ := s.store.CountApprovedPromoters()
	payable, _ := s.store.PayableTotal()
	return map[string]any{
		"products": products, "pending": pending, "promoters": promoters,
		"payableTotal": payable,
	}
}

// PublicStats counts the anonymous landing numbers: recruiting
// listings, approved promoters, and registered post URLs.
func (s *Service) PublicStats() map[string]any {
	recruiting, _ := s.store.CountConsignmentProducts(store.ConsignRecruiting)
	promoters, _ := s.store.CountApprovedPromoters()
	posts, _ := s.store.CountConsignmentPosts()
	return map[string]any{
		"recruiting": recruiting, "promoters": promoters, "posts": posts,
	}
}

// WorkItem is one approved application with its materials, posts, and count.
type WorkItem struct {
	ApplicationView
	Photo           string                   `json:"photo"`
	Points          string                   `json:"points"`
	BannedWords     string                   `json:"bannedWords"`
	Materials       string                   `json:"materials"`
	Posts           []*store.ConsignmentPost `json:"posts"`
	ConfirmedOrders int                      `json:"confirmedOrders"`
}

// WorkView is the W0 screen payload: approved items plus earnings.
type WorkView struct {
	Items  []*WorkItem `json:"items"`
	Orders int         `json:"orders"`
	Total  int         `json:"total"`
}

// MyWork returns one member's workspace: approved items with materials,
// registered posts, and confirmed earnings.
func (s *Service) MyWork(memberID int64) *WorkView {
	rows, err := s.store.ListMemberApplications(memberID)
	out := &WorkView{Items: []*WorkItem{}}
	if err != nil {
		return out
	}
	for _, a := range rows {
		if a.Status != store.ApplyApproved {
			continue
		}
		v, err := s.applicationView(a)
		if err != nil {
			continue
		}
		item := &WorkItem{ApplicationView: *v, ConfirmedOrders: a.ConfirmedOrders,
			Posts: []*store.ConsignmentPost{}}
		if p, err := s.store.GetConsignmentProduct(a.ProductID); err == nil {
			item.Photo, item.Points = p.Photo, p.Points
			item.BannedWords, item.Materials = p.BannedWords, p.Materials
		}
		if posts, err := s.store.ListConsignmentPosts(a.ID); err == nil {
			item.Posts = posts
		}
		out.Items = append(out.Items, item)
	}
	out.Orders, out.Total, _ = s.store.MemberEarnings(memberID)
	return out
}

// RegisterPost records one published URL for the approved owner.
func (s *Service) RegisterPost(memberID, appID int64, url string) (*store.ConsignmentPost, error) {
	url = strings.TrimSpace(url)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, ErrBadURL
	}
	a, err := s.store.GetApplication(appID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoApplication
		}
		return nil, err
	}
	if a.MemberID != memberID || a.Status != store.ApplyApproved {
		return nil, ErrForbidden
	}
	return s.store.AddConsignmentPost(appID, url)
}

// ConfirmOrders records the admin-verified order count (W25 manual).
// Counts only move forward: lowering needs a conscious reset to zero.
func (s *Service) ConfirmOrders(appID int64, orders int) error {
	if orders < 0 {
		return ErrBadCount
	}
	a, err := s.store.GetApplication(appID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrNoApplication
		}
		return err
	}
	if a.Status != store.ApplyApproved {
		return ErrBadDecision
	}
	return s.store.SetConfirmedOrders(appID, orders)
}
