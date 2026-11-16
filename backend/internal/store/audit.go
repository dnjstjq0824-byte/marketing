package store

// ---------------------------------------------------------------------------
// Operator audit log (W20-2): who did what, and when. Work-scoped rows
// carry the product id in WorkID so the S3-D screen shows one work's
// human history; team/account/settings rows keep WorkID zero.
// ---------------------------------------------------------------------------

// Audit actions. Every value is stored verbatim; the console maps them
// to screen phrases (labels.js AUDIT_ACTION).
const (
	AuditWorkPause         = "work_pause"
	AuditWorkResume        = "work_resume"
	AuditWorkDelete        = "work_delete"
	AuditPostHold          = "post_hold"
	AuditPostResume        = "post_resume"
	AuditTeamMembers       = "team_members"
	AuditTeamUpdate        = "team_update"
	AuditAccountUnregister = "account_unregister"
	AuditSettingsChange    = "settings_change"
	AuditConfirmPublished  = "confirm_published"
	AuditConfirmMissing    = "confirm_missing"
)

// AuditEntry is one human operation.
type AuditEntry struct {
	ID        int64  `json:"id"`
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	RefType   string `json:"refType"`
	RefID     int64  `json:"refId"`
	WorkID    int64  `json:"workId"`
	Detail    string `json:"detail"`
	CreatedAt string `json:"createdAt"`
}

// AppendAudit records one human operation. An empty actor is stored as
// "system" (non-interactive callers); user code passes the operator name.
func (s *Store) AppendAudit(actor, action, refType string, refID, workID int64, detail string) (*AuditEntry, error) {
	if actor == "" {
		actor = "system"
	}
	at := nowUTC()
	res, err := s.db.Exec(`INSERT INTO audit_log(actor,action,ref_type,ref_id,work_id,detail,created_at)
		VALUES(?,?,?,?,?,?,?)`, actor, action, refType, refID, workID, detail, at)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &AuditEntry{ID: id, Actor: actor, Action: action, RefType: refType, RefID: refID, WorkID: workID, Detail: detail, CreatedAt: at}, nil
}

// ListAuditByWork returns one work's human history, oldest first.
func (s *Store) ListAuditByWork(workID int64, limit int) ([]*AuditEntry, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.ro.Query(`SELECT id,actor,action,ref_type,ref_id,work_id,detail,created_at
		FROM audit_log WHERE work_id=? ORDER BY id ASC LIMIT ?`, workID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.RefType, &e.RefID, &e.WorkID, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// ListAuditRecent returns the newest audit rows across every scope.
func (s *Store) ListAuditRecent(limit int) ([]*AuditEntry, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.ro.Query(`SELECT id,actor,action,ref_type,ref_id,work_id,detail,created_at
		FROM audit_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.RefType, &e.RefID, &e.WorkID, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}
