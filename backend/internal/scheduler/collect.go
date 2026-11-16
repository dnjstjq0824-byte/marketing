package scheduler

import (
	"encoding/json"
	"time"

	"somun/internal/store"
)

// CollectDue enqueues one P-lane collect job per due (published URL, window)
// pair (W7). The collect_windows ledger makes every tick idempotent: already
// covered windows are skipped, and the UNIQUE key absorbs races. It returns
// the number of jobs enqueued.
func (s *Service) CollectDue(now time.Time) (int, error) {
	settings, err := store.GetAutomationSettings(s.store.DB())
	if err != nil {
		return 0, err
	}
	if !settings.Enabled {
		return 0, nil
	}
	due, err := s.store.DueCollects(now)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, d := range due {
		payload, _ := json.Marshal(map[string]any{
			"content_id": d.ContentID, "product_id": d.ProductID,
			"channel": d.Channel, "platform": d.Platform,
			"account": d.Account, "url": d.URL, "window": d.Window,
		})
		// Literal "collect": importing queue here would cycle
		// (queue -> scheduler -> queue is forbidden).
		job, err := s.store.CreateJobFull("collect", d.ContentID, string(payload), d.Business, d.Priority, false)
		if err != nil {
			return n, err
		}
		if team, terr := s.store.EffectiveProductTeam(d.ProductID); terr == nil {
			_ = s.store.SetJobTeam(job.ID, team)
		}
		// Another tick may have covered this window first: the ledger
		// decides, and the loser job is failed (never silently dropped)
		// so the queue screen explains itself.
		fresh, err := s.store.EnsureCollectWindow(d.ContentID, d.Window, d.Account, job.ID)
		if err != nil {
			return n, err
		}
		if !fresh {
			// W20-5 screen phrase; queue.UserErrorMessage maps the
			// same marker, but importing queue here would cycle.
			_, _ = s.store.SetJobFailed(job.ID, "수집이 겹쳐 건너뛰었습니다")
			continue
		}
		n++
	}
	return n, nil
}
