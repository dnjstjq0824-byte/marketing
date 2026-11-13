package store

// ---------------------------------------------------------------------------
// Chat messages (v10): console/worker AI conversations per account.
// ---------------------------------------------------------------------------

// ChatMessage is one recorded turn.
type ChatMessage struct {
	ID        int64  `json:"id"`
	AccountID int64  `json:"accountId"`
	Role      string `json:"role"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
}

// AppendChatMessage records one turn.
func (s *Store) AppendChatMessage(accountID int64, role, text string) (*ChatMessage, error) {
	res, err := s.db.Exec(`INSERT INTO chat_messages(account_id,role,text,created_at)
		VALUES(?,?,?,?)`, accountID, role, text, nowUTC())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &ChatMessage{ID: id, AccountID: accountID, Role: role, Text: text}, nil
}

// ListChatHistory returns the newest turns, oldest first, capped at limit.
func (s *Store) ListChatHistory(accountID int64, limit int) ([]*ChatMessage, error) {
	if limit < 1 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id,account_id,role,text,created_at FROM chat_messages
		WHERE account_id=? ORDER BY id DESC LIMIT ?`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ChatMessage
	for rows.Next() {
		m := &ChatMessage{}
		if err := rows.Scan(&m.ID, &m.AccountID, &m.Role, &m.Text, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}
