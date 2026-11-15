package store

import (
	"database/sql"
	"fmt"
)

// Channel platform registry statuses (W21-1).
const (
	PlatformReady     = "ready"
	PlatformPreparing = "preparing"
)

// ChannelPlatform is one row of the channel_platforms registry.
type ChannelPlatform struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	Group      string `json:"group"`
	LoginURL   string `json:"loginUrl"`
	TextRules  string `json:"textRules"`
	MediaRules string `json:"mediaRules"`
	Status     string `json:"status"`
}

// ListChannelPlatforms returns the registry in key order.
func (s *Store) ListChannelPlatforms() ([]*ChannelPlatform, error) {
	rows, err := s.ro.Query(`SELECT key,name,grp,login_url,text_rules,media_rules,status
		FROM channel_platforms ORDER BY key ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ChannelPlatform
	for rows.Next() {
		var p ChannelPlatform
		if err := rows.Scan(&p.Key, &p.Name, &p.Group, &p.LoginURL,
			&p.TextRules, &p.MediaRules, &p.Status); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// GetChannelPlatform fetches one registry row by key.
func (s *Store) GetChannelPlatform(key string) (*ChannelPlatform, error) {
	var p ChannelPlatform
	err := s.ro.QueryRow(`SELECT key,name,grp,login_url,text_rules,media_rules,status
		FROM channel_platforms WHERE key=?`, key).Scan(
		&p.Key, &p.Name, &p.Group, &p.LoginURL, &p.TextRules, &p.MediaRules, &p.Status)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// SetChannelPlatformStatus flips ready/preparing. Anything else is rejected.
func (s *Store) SetChannelPlatformStatus(key, status string) error {
	if status != PlatformReady && status != PlatformPreparing {
		return fmt.Errorf("bad platform status %q", status)
	}
	res, err := s.db.Exec(`UPDATE channel_platforms SET status=? WHERE key=?`, status, key)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
