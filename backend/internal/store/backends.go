package store

import "database/sql"

// PublishBackend is one W22 execution path: direct session posting or a
// probe-proven agent relay. Ready flips only after a supervised live
// probe pins that backend's approval path.
type PublishBackend struct {
	Name       string `json:"name"`
	AIPlatform string `json:"aiPlatform"`
	Ready      bool   `json:"ready"`
}

// ListPublishBackends returns the registry by name.
func (s *Store) ListPublishBackends() ([]*PublishBackend, error) {
	rows, err := s.ro.Query(`SELECT name, ai_platform, ready FROM publish_backends ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*PublishBackend{}
	for rows.Next() {
		b := &PublishBackend{}
		var ready int
		if err := rows.Scan(&b.Name, &b.AIPlatform, &ready); err != nil {
			return nil, err
		}
		b.Ready = ready != 0
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetPublishBackend loads one backend by name.
func (s *Store) GetPublishBackend(name string) (*PublishBackend, error) {
	b := &PublishBackend{}
	var ready int
	err := s.ro.QueryRow(`SELECT name, ai_platform, ready FROM publish_backends WHERE name=?`, name).
		Scan(&b.Name, &b.AIPlatform, &ready)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	b.Ready = ready != 0
	return b, nil
}

// SetPublishBackendReady flips one backend's probe gate.
func (s *Store) SetPublishBackendReady(name string, ready bool) error {
	val := 0
	if ready {
		val = 1
	}
	res, err := s.db.Exec(`UPDATE publish_backends SET ready=? WHERE name=?`, val, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
