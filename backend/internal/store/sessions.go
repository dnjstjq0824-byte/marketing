package store

import (
	"database/sql"
	"errors"
)

// ---------------------------------------------------------------------------
// Operator sessions (v9): DB-backed so logins survive restarts.
// ---------------------------------------------------------------------------

// CreateOperatorSession records a hashed bearer token.
func (s *Store) CreateOperatorSession(tokenHash string, userID int64, username, expiresAt string) error {
	_, err := s.db.Exec(`INSERT INTO operator_sessions(token_hash,user_id,username,expires_at,created_at)
		VALUES(?,?,?,?,?)`, tokenHash, userID, username, expiresAt, nowUTC())
	return err
}

// OperatorSession is one persisted login.
type OperatorSession struct {
	UserID    int64
	Username  string
	ExpiresAt string
}

// GetOperatorSession reads one session by token hash.
func (s *Store) GetOperatorSession(tokenHash string) (*OperatorSession, error) {
	var out OperatorSession
	err := s.db.QueryRow(`SELECT user_id,username,expires_at FROM operator_sessions WHERE token_hash=?`,
		tokenHash).Scan(&out.UserID, &out.Username, &out.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &out, nil
}

// DeleteOperatorSession drops one token (logout, expiry).
func (s *Store) DeleteOperatorSession(tokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM operator_sessions WHERE token_hash=?`, tokenHash)
	return err
}
