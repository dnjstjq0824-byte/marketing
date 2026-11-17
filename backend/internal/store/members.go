package store

import "database/sql"

// Member is one public consignment user. PasswordHash never serializes.
type Member struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	Nickname     string `json:"nickname"`
	CreatedAt    string `json:"createdAt"`
}

// CreateMember inserts one member. Duplicate emails return ErrDuplicate.
func (s *Store) CreateMember(email, hash, nickname string) (*Member, error) {
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO members(email, password_hash, nickname, created_at)
		VALUES(?,?,?,?)`, email, hash, nickname, now)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetMember(id)
}

// GetMember loads one member by id.
func (s *Store) GetMember(id int64) (*Member, error) {
	m := &Member{}
	err := s.ro.QueryRow(`SELECT id, email, password_hash, nickname, created_at
		FROM members WHERE id=?`, id).Scan(&m.ID, &m.Email, &m.PasswordHash, &m.Nickname, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return m, err
}

// GetMemberByEmail loads one member by its unique email.
func (s *Store) GetMemberByEmail(email string) (*Member, error) {
	m := &Member{}
	err := s.ro.QueryRow(`SELECT id, email, password_hash, nickname, created_at
		FROM members WHERE email=?`, email).Scan(&m.ID, &m.Email, &m.PasswordHash, &m.Nickname, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return m, err
}

// MemberSession is one login token row.
type MemberSession struct {
	TokenHash string
	MemberID  int64
	ExpiresAt string
}

// CreateMemberSession stores one login token.
func (s *Store) CreateMemberSession(hash string, memberID int64, expires string) error {
	_, err := s.db.Exec(`INSERT INTO member_sessions(token_hash, member_id, expires_at)
		VALUES(?,?,?)
		ON CONFLICT(token_hash) DO UPDATE SET member_id=excluded.member_id, expires_at=excluded.expires_at`,
		hash, memberID, expires)
	return err
}

// GetMemberSession loads one session by token hash.
func (s *Store) GetMemberSession(hash string) (*MemberSession, error) {
	ms := &MemberSession{}
	err := s.ro.QueryRow(`SELECT token_hash, member_id, expires_at FROM member_sessions
		WHERE token_hash=?`, hash).Scan(&ms.TokenHash, &ms.MemberID, &ms.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return ms, err
}

// TouchMemberSession extends one session's expiry.
func (s *Store) TouchMemberSession(hash, expires string) error {
	_, err := s.db.Exec(`UPDATE member_sessions SET expires_at=? WHERE token_hash=?`, expires, hash)
	return err
}

// DeleteMemberSession drops one token.
func (s *Store) DeleteMemberSession(hash string) error {
	_, err := s.db.Exec(`DELETE FROM member_sessions WHERE token_hash=?`, hash)
	return err
}
