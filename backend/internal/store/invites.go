package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// Beta invite codes (open-beta gate): signup consumes one use.
var (
	ErrNoInvite     = errors.New("invite code not found")
	ErrInviteSpent  = errors.New("invite code already used up")
	ErrInviteRevoked = errors.New("invite code revoked")
)

// Invite is one row of the invite_codes registry.
type Invite struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	MaxUses   int    `json:"maxUses"`
	Used      int    `json:"used"`
	CreatedBy string `json:"createdBy"`
	CreatedAt string `json:"createdAt"`
	Revoked   bool   `json:"revoked"`
}

const inviteCols = `id,code,max_uses,used,created_by,created_at,revoked`

func scanInvite(row interface{ Scan(...any) error }) (*Invite, error) {
	var v Invite
	var revoked int
	if err := row.Scan(&v.ID, &v.Code, &v.MaxUses, &v.Used, &v.CreatedBy, &v.CreatedAt, &revoked); err != nil {
		return nil, err
	}
	v.Revoked = revoked != 0
	return &v, nil
}

// CreateInvite stores one operator-minted code.
func (s *Store) CreateInvite(code string, maxUses int, by string) (*Invite, error) {
	if maxUses < 1 {
		maxUses = 1
	}
	res, err := s.db.Exec(`INSERT INTO invite_codes(code,max_uses,used,created_by,created_at,revoked)
		VALUES(?,?,0,?,?,0)`, code, maxUses, by, nowUTC())
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetInviteByID(id)
}

// GetInviteByID reads one code by id.
func (s *Store) GetInviteByID(id int64) (*Invite, error) {
	v, err := scanInvite(s.ro.QueryRow(`SELECT `+inviteCols+` FROM invite_codes WHERE id=?`, id))
	if err != nil {
		return nil, err
	}
	return v, nil
}

// GetInvite reads one code by its code string.
func (s *Store) GetInvite(code string) (*Invite, error) {
	v, err := scanInvite(s.ro.QueryRow(`SELECT `+inviteCols+` FROM invite_codes WHERE code=?`, code))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoInvite
		}
		return nil, err
	}
	return v, nil
}

// ListInvites returns codes newest first.
func (s *Store) ListInvites() ([]*Invite, error) {
	rows, err := s.ro.Query(`SELECT ` + inviteCols + ` FROM invite_codes ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Invite
	for rows.Next() {
		v, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// RevokeInvite disables one code (uses keep their history).
func (s *Store) RevokeInvite(id int64) error {
	res, err := s.db.Exec(`UPDATE invite_codes SET revoked=1 WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNoInvite
	}
	return nil
}

// ConsumeInvite spends one use atomically. Lost races report spent.
func (s *Store) ConsumeInvite(code string) error {
	res, err := s.db.Exec(`UPDATE invite_codes SET used=used+1
		WHERE code=? AND revoked=0 AND used<max_uses`, code)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 1 {
		return nil
	}
	v, err := s.GetInvite(code)
	if err != nil {
		return err
	}
	if v.Revoked {
		return ErrInviteRevoked
	}
	return fmt.Errorf("%w", ErrInviteSpent)
}
