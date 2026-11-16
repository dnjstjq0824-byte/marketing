package store

import (
	"database/sql"
	"time"
)

// ---------------------------------------------------------------------------
// System health snapshot (W20-4): one screen's worth of DB, disk,
// executor, queue, and error state for GET /api/system/health.
// ---------------------------------------------------------------------------

// DiskLowPct is the free-space alert line: below it the health endpoint
// files a disk_low notification (media fills disks fast).
const DiskLowPct = 10.0

// HealthDB is the database reachability probe.
type HealthDB struct {
	OK        bool  `json:"ok"`
	LatencyMs int64 `json:"latencyMs"`
}

// HealthDisk is the data-volume free space reading.
type HealthDisk struct {
	Path       string  `json:"path"`
	FreeBytes  uint64  `json:"freeBytes"`
	TotalBytes uint64  `json:"totalBytes"`
	FreePct    float64 `json:"freePct"`
	Low        bool    `json:"low"`
}

// HealthAgent is one executor's connectivity and current load.
type HealthAgent struct {
	ID         int64  `json:"id"`
	Label      string `json:"label"`
	Host       string `json:"host"`
	LastSeenAt string `json:"lastSeenAt"`
	Online     bool   `json:"online"`
	Active     int    `json:"active"`
	Capacity   int    `json:"capacity"`
}

// HealthPCLoad aggregates active claims per host PC.
type HealthPCLoad struct {
	Host     string `json:"host"`
	Active   int    `json:"active"`
	Capacity int    `json:"capacity"`
}

// HealthTeamQueue is one team's pending backlog.
type HealthTeamQueue struct {
	TeamID  int64  `json:"teamId"`
	Team    string `json:"team"`
	Pending int    `json:"pending"`
}

// HealthError is one recent failure for the error tail.
type HealthError struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	RefID int64  `json:"refId"`
	Error string `json:"error"`
	At    string `json:"at"`
}

// ProbeDB pings the database and measures the round trip.
func (s *Store) ProbeDB() HealthDB {
	start := time.Now()
	if err := s.db.Ping(); err != nil {
		return HealthDB{OK: false}
	}
	return HealthDB{OK: true, LatencyMs: time.Since(start).Milliseconds()}
}

// AgentHealth lists every executor with its online verdict (same 3-minute
// cutoff as the agent stats) and live claimed count.
func (s *Store) AgentHealth(now time.Time) ([]*HealthAgent, error) {
	agents, err := s.ListAgents(AgentFilter{}, 200, 0)
	if err != nil {
		return nil, err
	}
	cutoff := now.UTC().Add(-OnlineCutoffMin * time.Minute).Format(time.RFC3339)
	out := []*HealthAgent{}
	for _, a := range agents {
		active, err := s.ClaimedCountByAgent(a.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, &HealthAgent{
			ID: a.ID, Label: a.Label, Host: a.Host, LastSeenAt: a.LastSeenAt,
			Online: a.Enabled && a.LastSeenAt != "" && a.LastSeenAt >= cutoff,
			Active: active, Capacity: a.Capacity,
		})
	}
	return out, nil
}

// PCLoad folds agent health rows into per-host load. Empty hosts group
// under "local" (agents that never reported a host name).
func PCLoad(agents []*HealthAgent) []*HealthPCLoad {
	byHost := map[string]*HealthPCLoad{}
	var order []string
	for _, a := range agents {
		host := a.Host
		if host == "" {
			host = "local"
		}
		row, ok := byHost[host]
		if !ok {
			row = &HealthPCLoad{Host: host}
			byHost[host] = row
			order = append(order, host)
		}
		row.Active += a.Active
		row.Capacity += a.Capacity
	}
	out := []*HealthPCLoad{}
	for _, host := range order {
		out = append(out, byHost[host])
	}
	return out
}

// TeamPendingCounts returns the pending backlog per team, oldest team
// first. Jobs without a team count toward the default team.
func (s *Store) TeamPendingCounts() ([]*HealthTeamQueue, error) {
	var defID int64
	var defName string
	_ = s.ro.QueryRow(`SELECT id,name FROM teams WHERE is_default=1`).Scan(&defID, &defName)
	rows, err := s.ro.Query(`SELECT j.team_id, COALESCE(t.name,''), COUNT(*)
		FROM jobs j LEFT JOIN teams t ON t.id=j.team_id
		WHERE j.status='pending' GROUP BY j.team_id ORDER BY j.team_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	merged := map[int64]*HealthTeamQueue{}
	var order []int64
	for rows.Next() {
		var team sql.NullInt64
		var name string
		var n int
		if err := rows.Scan(&team, &name, &n); err != nil {
			return nil, err
		}
		id := defID
		label := defName
		if team.Valid {
			id = team.Int64
			label = name
		}
		row, ok := merged[id]
		if !ok {
			row = &HealthTeamQueue{TeamID: id, Team: label}
			merged[id] = row
			order = append(order, id)
		}
		row.Pending += n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []*HealthTeamQueue{}
	for _, id := range order {
		out = append(out, merged[id])
	}
	return out, nil
}

// RecentErrors returns the newest failed jobs, capped at limit.
func (s *Store) RecentErrors(limit int) ([]*HealthError, error) {
	if limit < 1 || limit > 50 {
		limit = 10
	}
	rows, err := s.ro.Query(`SELECT id,type,ref_id,error,COALESCE(NULLIF(finished_at,''),created_at)
		FROM jobs WHERE status='failed' ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*HealthError{}
	for rows.Next() {
		var e HealthError
		if err := rows.Scan(&e.ID, &e.Type, &e.RefID, &e.Error, &e.At); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// DiskUsage reads the data-volume free space. Unknown platforms report
// zero totals, which never trips the low-space alert.
func DiskUsage(path string) HealthDisk {
	free, total := diskFreeTotal(path)
	d := HealthDisk{Path: path, FreeBytes: free, TotalBytes: total}
	if total > 0 {
		d.FreePct = float64(free) * 100 / float64(total)
		d.Low = d.FreePct < DiskLowPct
	}
	return d
}
