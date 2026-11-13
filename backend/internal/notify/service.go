// Package notify is the local outbox (D30): the console badge reads it,
// nothing is sent anywhere external.
package notify

import (
	"fmt"
	"sync"
	"time"

	"somun/internal/store"
)

// Kinds match the notifications CHECK constraint.
const (
	KindJobFailed     = "job_failed"
	KindJobStuck      = "job_stuck"
	KindLeaseExpired  = "lease_expired"
	KindSessionExpire = "session_expired"
	KindSessionLimit  = "session_limited"
	KindPublishFail   = "publish_failed"
	KindBackupFail    = "backup_failed"
	KindOverdueHold   = "overdue_hold"
	KindReviewHuman   = "review_human"
)

var severityByKind = map[string]string{
	KindJobFailed:   "error",
	KindPublishFail: "error",
	KindBackupFail:  "error",
}

func severityFor(kind string) string {
	if s, ok := severityByKind[kind]; ok {
		return s
	}
	return "warn"
}

// Service files notifications with unread-term dedup.
type Service struct {
	store *store.Store
	mu    sync.Mutex
}

// NewService builds the outbox over a store.
func NewService(st *store.Store) *Service { return &Service{store: st} }

// Notify files one row unless the same unread dedup key exists.
// It reports whether a row was created.
func (s *Service) Notify(kind, refType string, refID int64, message string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s:%s:%d", kind, refType, refID)
	return s.store.CreateNotificationDedup(kind, severityFor(kind), refType, refID, key, message)
}

// JobFailed files a worker-failure notice.
func (s *Service) JobFailed(jobID int64, jobType, reason string) (bool, error) {
	return s.Notify(KindJobFailed, "job", jobID, fmt.Sprintf("작업 #%d(%s) 실패: %s", jobID, jobType, reason))
}

// SessionState files expired/limited transitions. Other moves stay silent.
func (s *Service) SessionState(accountID int64, name, prev, next string) (bool, error) {
	if prev == next {
		return false, nil
	}
	switch next {
	case "expired":
		return s.Notify(KindSessionExpire, "account", accountID, fmt.Sprintf("계정 %s: 세션 만료 — 재로그인 필요", name))
	case "limited":
		return s.Notify(KindSessionLimit, "account", accountID, fmt.Sprintf("계정 %s: 사용 한도 — 발행 정지", name))
	default:
		return false, nil
	}
}

// BackupFailed files a backup-failure notice (system scope, ref 0).
func (s *Service) BackupFailed(backupErr error) (bool, error) {
	return s.Notify(KindBackupFail, "system", 0, fmt.Sprintf("백업 실패: %v", backupErr))
}

// ReviewHuman files a needs-a-person review notice.
func (s *Service) ReviewHuman(contentID int64, message string) (bool, error) {
	return s.Notify(KindReviewHuman, "content", contentID, message)
}

// Sweep detects stuck jobs (30m+) and day-overdue holds, filing one unread
// notice per target. Repeats dedup to the single unread row.
func (s *Service) Sweep() (stuck, overdue int, err error) {
	jobs, err := s.store.ListStuckClaimed(store.StuckClaimedMin)
	if err != nil {
		return 0, 0, err
	}
	for _, j := range jobs {
		made, err := s.Notify(KindJobStuck, "job", j.ID, fmt.Sprintf("작업 #%d(%s): 30분 넘게 처리 중", j.ID, j.Type))
		if err != nil {
			return stuck, overdue, err
		}
		if made {
			stuck++
		}
	}
	cutoff := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	holds, err := s.store.ListOverdueHolds(cutoff)
	if err != nil {
		return stuck, overdue, err
	}
	for _, h := range holds {
		made, err := s.Notify(KindOverdueHold, "content", h.ID, fmt.Sprintf("콘텐츠 #%d(%s): 1일 넘게 보류", h.ID, h.Account))
		if err != nil {
			return stuck, overdue, err
		}
		if made {
			overdue++
		}
	}
	return stuck, overdue, nil
}

// List returns newest-first rows plus the unread count for the badge.
func (s *Service) List(unreadOnly bool, limit, offset int) ([]*store.Notification, int, error) {
	items, err := s.store.ListNotifications(unreadOnly, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	n, err := s.store.CountUnreadNotifications()
	if err != nil {
		return nil, 0, err
	}
	return items, n, nil
}

// Read marks one notice read, reporting whether it existed.
func (s *Service) Read(id int64) (bool, error) { return s.store.MarkNotificationRead(id) }

// ReadAll marks every unread notice read and reports how many.
func (s *Service) ReadAll() (int64, error) { return s.store.MarkAllNotificationsRead() }
