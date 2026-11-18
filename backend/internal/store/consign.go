package store

import "database/sql"

// Consignment product lifecycle.
const (
	ConsignRecruiting = "recruiting"
	ConsignClosed     = "closed"
)

// Consignment application states.
const (
	ApplyPending  = "pending"
	ApplyApproved = "approved"
	ApplyRejected = "rejected"
)

// ConsignmentProduct is one promotable listing. PrivateURLEnc holds
// ciphertext and never serializes; serve it only through the gated
// private-link endpoint after an approval check.
type ConsignmentProduct struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Store         string `json:"store"`
	Price         int    `json:"price"`
	Photo         string `json:"photo"`
	Facts         string `json:"facts"`
	Points        string `json:"points"`
	BannedWords   string `json:"bannedWords"`
	Materials     string `json:"materials"`
	FeePerOrder   int    `json:"feePerOrder"`
	SettleTerms   string `json:"settleTerms"`
	PrivateURLEnc string `json:"-"`
	Category      string `json:"category"`
	RecruitUntil  string `json:"recruitUntil"`
	Status        string `json:"status"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

const consignCols = `id,name,store,price,photo,facts,points,banned_words,materials,fee_per_order,settle_terms,private_url_enc,category,recruit_until,status,created_at,updated_at`

func scanConsignProduct(row interface{ Scan(...any) error }) (*ConsignmentProduct, error) {
	p := &ConsignmentProduct{}
	err := row.Scan(&p.ID, &p.Name, &p.Store, &p.Price, &p.Photo, &p.Facts,
		&p.Points, &p.BannedWords, &p.Materials, &p.FeePerOrder, &p.SettleTerms,
		&p.PrivateURLEnc, &p.Category, &p.RecruitUntil, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// CreateConsignmentProduct inserts one listing.
func (s *Store) CreateConsignmentProduct(p *ConsignmentProduct) (*ConsignmentProduct, error) {
	now := nowUTC()
	status := p.Status
	if status == "" {
		status = ConsignClosed
	}
	res, err := s.db.Exec(`INSERT INTO consignment_products(name,store,price,photo,facts,points,
		banned_words,materials,fee_per_order,settle_terms,private_url_enc,category,recruit_until,status,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, p.Name, p.Store, p.Price, p.Photo, p.Facts,
		p.Points, p.BannedWords, p.Materials, p.FeePerOrder, p.SettleTerms,
		p.PrivateURLEnc, p.Category, p.RecruitUntil, status, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetConsignmentProduct(id)
}

// GetConsignmentProduct loads one listing by id.
func (s *Store) GetConsignmentProduct(id int64) (*ConsignmentProduct, error) {
	p, err := scanConsignProduct(s.ro.QueryRow(`SELECT `+consignCols+` FROM consignment_products WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return p, err
}

// ListConsignmentProducts returns listings, newest first. Empty status
// lists all; otherwise only that status.
func (s *Store) ListConsignmentProducts(status string) ([]*ConsignmentProduct, error) {
	query := `SELECT ` + consignCols + ` FROM consignment_products`
	var rows *sql.Rows
	var err error
	if status == "" {
		rows, err = s.ro.Query(query + ` ORDER BY id DESC`)
	} else {
		rows, err = s.ro.Query(query+` WHERE status=? ORDER BY id DESC`, status)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*ConsignmentProduct{}
	for rows.Next() {
		p, err := scanConsignProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpdateConsignmentProduct rewrites one listing by id.
func (s *Store) UpdateConsignmentProduct(p *ConsignmentProduct) error {
	res, err := s.db.Exec(`UPDATE consignment_products SET name=?,store=?,price=?,photo=?,facts=?,points=?,
		banned_words=?,materials=?,fee_per_order=?,settle_terms=?,private_url_enc=?,category=?,recruit_until=?,status=?,updated_at=?
		WHERE id=?`, p.Name, p.Store, p.Price, p.Photo, p.Facts, p.Points,
		p.BannedWords, p.Materials, p.FeePerOrder, p.SettleTerms,
		p.PrivateURLEnc, p.Category, p.RecruitUntil, p.Status, nowUTC(), p.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountConsignmentProducts counts listings, all or by status.
func (s *Store) CountConsignmentProducts(status string) (int, error) {
	var n int
	var err error
	if status == "" {
		err = s.ro.QueryRow(`SELECT COUNT(*) FROM consignment_products`).Scan(&n)
	} else {
		err = s.ro.QueryRow(`SELECT COUNT(*) FROM consignment_products WHERE status=?`, status).Scan(&n)
	}
	return n, err
}

// ConsignmentApplication is one member's request to promote a product.
// FeeAgreed snapshots the per-order fee at apply time. ConfirmedOrders is
// the admin-verified order count feeding earnings (W25 manual confirm).
type ConsignmentApplication struct {
	ID              int64  `json:"id"`
	ProductID       int64  `json:"productId"`
	MemberID        int64  `json:"memberId"`
	Channels        string `json:"channels"`
	Plan            string `json:"plan"`
	FeeAgreed       int    `json:"feeAgreed"`
	Status          string `json:"status"`
	Reason          string `json:"reason"`
	ConfirmedOrders int    `json:"confirmedOrders"`
	CreatedAt       string `json:"createdAt"`
}

const consignAppCols = `id,product_id,member_id,channels,plan,fee_agreed,status,reason,confirmed_orders,created_at`

func scanConsignApp(row interface{ Scan(...any) error }) (*ConsignmentApplication, error) {
	a := &ConsignmentApplication{}
	err := row.Scan(&a.ID, &a.ProductID, &a.MemberID, &a.Channels, &a.Plan,
		&a.FeeAgreed, &a.Status, &a.Reason, &a.ConfirmedOrders, &a.CreatedAt)
	return a, err
}

// CreateApplication inserts one request. Repeat applies return ErrDuplicate.
func (s *Store) CreateApplication(a *ConsignmentApplication) (*ConsignmentApplication, error) {
	res, err := s.db.Exec(`INSERT INTO consignment_applications(product_id,member_id,channels,plan,
		fee_agreed,status,reason,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		a.ProductID, a.MemberID, a.Channels, a.Plan, a.FeeAgreed, ApplyPending, "", nowUTC())
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetApplication(id)
}

// GetApplication loads one request by id.
func (s *Store) GetApplication(id int64) (*ConsignmentApplication, error) {
	a, err := scanConsignApp(s.ro.QueryRow(`SELECT `+consignAppCols+` FROM consignment_applications WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return a, err
}

// ListApplications returns requests, newest first. Empty status lists all.
func (s *Store) ListApplications(status string) ([]*ConsignmentApplication, error) {
	query := `SELECT ` + consignAppCols + ` FROM consignment_applications`
	var rows *sql.Rows
	var err error
	if status == "" {
		rows, err = s.ro.Query(query + ` ORDER BY id DESC`)
	} else {
		rows, err = s.ro.Query(query+` WHERE status=? ORDER BY id DESC`, status)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*ConsignmentApplication{}
	for rows.Next() {
		a, err := scanConsignApp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListMemberApplications returns one member's requests, newest first.
func (s *Store) ListMemberApplications(memberID int64) ([]*ConsignmentApplication, error) {
	rows, err := s.ro.Query(`SELECT `+consignAppCols+` FROM consignment_applications
		WHERE member_id=? ORDER BY id DESC`, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*ConsignmentApplication{}
	for rows.Next() {
		a, err := scanConsignApp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DecideApplication flips one pending request to approved/rejected.
func (s *Store) DecideApplication(id int64, status, reason string) error {
	res, err := s.db.Exec(`UPDATE consignment_applications SET status=?, reason=? WHERE id=?`,
		status, reason, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountApplicationsByStatus counts requests in one state.
func (s *Store) CountApplicationsByStatus(status string) (int, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM consignment_applications WHERE status=?`, status).Scan(&n)
	return n, err
}

// CountApprovedPromoters counts distinct members holding an approval.
func (s *Store) CountApprovedPromoters() (int, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(DISTINCT member_id) FROM consignment_applications
		WHERE status='approved'`).Scan(&n)
	return n, err
}

// SetConfirmedOrders records the admin-verified order count (W25 manual).
func (s *Store) SetConfirmedOrders(appID int64, orders int) error {
	res, err := s.db.Exec(`UPDATE consignment_applications SET confirmed_orders=? WHERE id=?`,
		orders, appID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ConsignmentPost is one member-registered published URL.
type ConsignmentPost struct {
	ID            int64  `json:"id"`
	ApplicationID int64  `json:"applicationId"`
	URL           string `json:"url"`
	CreatedAt     string `json:"createdAt"`
}

// AddConsignmentPost records one published URL.
func (s *Store) AddConsignmentPost(appID int64, url string) (*ConsignmentPost, error) {
	res, err := s.db.Exec(`INSERT INTO consignment_posts(application_id, url, created_at)
		VALUES(?,?,?)`, appID, url, nowUTC())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	p := &ConsignmentPost{}
	err = s.ro.QueryRow(`SELECT id, application_id, url, created_at FROM consignment_posts
		WHERE id=?`, id).Scan(&p.ID, &p.ApplicationID, &p.URL, &p.CreatedAt)
	return p, err
}

// ListConsignmentPosts returns one application's URLs, oldest first.
func (s *Store) ListConsignmentPosts(appID int64) ([]*ConsignmentPost, error) {
	rows, err := s.ro.Query(`SELECT id, application_id, url, created_at FROM consignment_posts
		WHERE application_id=? ORDER BY id ASC`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*ConsignmentPost{}
	for rows.Next() {
		p := &ConsignmentPost{}
		if err := rows.Scan(&p.ID, &p.ApplicationID, &p.URL, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MemberEarnings sums confirmed orders × agreed fee over approved apps.
func (s *Store) MemberEarnings(memberID int64) (orders, total int, err error) {
	err = s.ro.QueryRow(`SELECT COALESCE(SUM(confirmed_orders),0),
		COALESCE(SUM(confirmed_orders*fee_agreed),0)
		FROM consignment_applications WHERE member_id=? AND status='approved'`,
		memberID).Scan(&orders, &total)
	return orders, total, err
}

// PayableTotal sums confirmed orders × agreed fee over all approvals.
func (s *Store) PayableTotal() (int, error) {
	var total int
	err := s.ro.QueryRow(`SELECT COALESCE(SUM(confirmed_orders*fee_agreed),0)
		FROM consignment_applications WHERE status='approved'`).Scan(&total)
	return total, err
}

// CountConsignmentPosts counts every member-registered published URL.
func (s *Store) CountConsignmentPosts() (int, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM consignment_posts`).Scan(&n)
	return n, err
}

// ApplicationCounts returns per-product applicant totals.
func (s *Store) ApplicationCounts() (map[int64]int, error) {
	rows, err := s.ro.Query(`SELECT product_id, COUNT(*) FROM consignment_applications GROUP BY product_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}
