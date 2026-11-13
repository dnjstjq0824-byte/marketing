package content

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"somun/internal/content/policy"
	"somun/internal/product"
	"somun/internal/store"
)

// Actors: only platform (human review actions) may change content status.
// The worker path creates drafts and nothing else.
const (
	ActorPlatform = "platform"
	ActorWorker   = "worker"
)

// Service errors.
var (
	ErrWorkerForbidden = errors.New("worker path cannot change content status")
	ErrUnknownPlatform = errors.New("unknown target platform")
	ErrPastSchedule    = errors.New("scheduled time must be in the future")
	ErrNoAccount       = errors.New("target account is required")
	ErrAILabelDenied   = errors.New("AI label is only allowed for virtual-persona contents")
	ErrBadPublishLog   = errors.New("only scheduled/held records can be created in tranche 1")
)

// autoModeValue mirrors session.PostingAuto. Importing the session package
// here would cycle (session imports content for resume), so the literal stays.
const autoModeValue = "auto"

// Service implements review transitions, scheduling, holds, and metrics.
type Service struct {
	store *store.Store
}

// NewService builds the content service.
func NewService(st *store.Store) *Service { return &Service{store: st} }

// Transition moves a content item along the state machine. Workers are
// rejected outright: status belongs to platform review actions only.
func (s *Service) Transition(id int64, to, actor string) (*store.Content, error) {
	if actor == ActorWorker {
		return nil, ErrWorkerForbidden
	}
	c, err := s.store.GetContent(id)
	if err != nil {
		return nil, err
	}
	if err := policy.ValidateTransition(c.Status, to); err != nil {
		return nil, err
	}
	if err := s.store.UpdateContentStatus(id, to); err != nil {
		return nil, err
	}
	return s.store.GetContent(id)
}

// Delete hard-deletes one content item with its publish logs and metrics.
func (s *Service) Delete(id int64) error {
	return s.store.DeleteContent(id)
}

// EditBody updates title/body. Disclosure lives in its own column and is
// never modified here, so reviewers cannot delete the compensation notice.
func (s *Service) EditBody(id int64, title, body string) (*store.Content, error) {
	if _, err := s.store.GetContent(id); err != nil {
		return nil, err
	}
	if err := s.store.UpdateContentBody(id, title, body); err != nil {
		return nil, err
	}
	return s.store.GetContent(id)
}

// SetPersona sets the virtual-persona flag. The AI label attaches only when
// the flag is true; any other combination is rejected.
func (s *Service) SetPersona(id int64, persona bool) (*store.Content, error) {
	if _, err := s.store.GetContent(id); err != nil {
		return nil, err
	}
	label := ""
	if persona {
		label = policy.AILabelText
	}
	if err := s.store.UpdateContentFlags(id, persona, label); err != nil {
		return nil, err
	}
	return s.store.GetContent(id)
}

// ScheduleInput is a publish reservation request.
type ScheduleInput struct {
	Account     string
	Platform    string
	ScheduledAt string // RFC3339; empty means unscheduled queue entry
}

// ScheduleResult is the reservation outcome with its serial queue position.
type ScheduleResult struct {
	Content       *store.Content `json:"content"`
	QueuePosition int            `json:"queuePosition"`
}

// Schedule reserves an item. Queued items schedule directly; pre-queued items
// auto-advance to queued only for auto-mode accounts (D16), otherwise they
// are rejected. Past times are rejected at input; platform and group must
// match the two-group structure.
func (s *Service) Schedule(id int64, in ScheduleInput) (*ScheduleResult, error) {
	c, err := s.store.GetContent(id)
	if err != nil {
		return nil, err
	}
	account := strings.TrimSpace(in.Account)
	if account == "" {
		return nil, ErrNoAccount
	}
	autoAdvance := false
	if c.Status != policy.StatusQueued {
		acc, err := s.store.GetAccountRowByName(account)
		if err != nil || acc.PostingMode != autoModeValue {
			return nil, errors.New("only queued items can be scheduled (auto-mode accounts auto-advance)")
		}
		autoAdvance = true
	}
	platform := strings.ToLower(strings.TrimSpace(in.Platform))
	group, ok := policy.GroupOf(platform)
	if !ok {
		return nil, ErrUnknownPlatform
	}
	scheduledAt := strings.TrimSpace(in.ScheduledAt)
	if scheduledAt != "" {
		t, err := time.Parse(time.RFC3339, scheduledAt)
		if err != nil {
			return nil, errors.New("scheduledAt must be RFC3339")
		}
		if !t.After(time.Now()) {
			return nil, ErrPastSchedule
		}
		scheduledAt = t.UTC().Format(time.RFC3339)
	}
	if autoAdvance {
		if err := s.store.UpdateContentStatus(id, policy.StatusQueued); err != nil {
			return nil, err
		}
	}
	if err := s.store.ScheduleContent(id, account, platform, group, scheduledAt); err != nil {
		return nil, err
	}
	if autoAdvance {
		// Record the platform transition; reschedules hit the duplicate
		// guard and keep the original record (idempotent by design).
		_, _ = s.store.CreatePublishLog(id, platform, account, "scheduled", "auto-advanced")
	}
	updated, err := s.store.GetContent(id)
	if err != nil {
		return nil, err
	}
	pos := 0
	if scheduledAt != "" {
		pos, err = s.store.QueuePosition(account, scheduledAt, id)
		if err != nil {
			return nil, err
		}
	}
	// P1-2: manual-group reservations feed the session-posting lane.
	if group == policy.GroupManual {
		if err := s.ensurePublishJob(updated, account, platform); err != nil {
			return nil, err
		}
	}
	return &ScheduleResult{Content: updated, QueuePosition: pos}, nil
}

// ensurePublishJob enqueues one P-lane job per content+account pair.
// Title/body ride the payload: the adapter has no content-read endpoint.
func (s *Service) ensurePublishJob(c *store.Content, account, platform string) error {
	active, err := s.store.HasActivePublishJob(c.ID, account)
	if err != nil || active {
		return err
	}
	business, priority := "", 0
	if p, err := s.store.GetProduct(c.ProductID); err == nil {
		business, priority = p.Business, p.Priority
	}
	payload, _ := json.Marshal(map[string]any{
		"content_id": c.ID, "account": account, "platform": platform,
		"title": c.Title, "body": c.Body,
	})
	// Literal "publish": importing queue here would cycle
	// (content -> queue -> agent -> session -> content).
	_, err = s.store.CreateJobFull("publish", c.ID, string(payload), business, priority, false)
	return err
}

// ResumeAccount releases an account's held items after re-login, except items
// overdue by a day or more, which stay held as report targets.
func (s *Service) ResumeAccount(account string) (resumed, overdue int, err error) {
	items, err := s.store.ListHeldByAccount(account)
	if err != nil {
		return 0, 0, err
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, it := range items {
		keep := false
		if it.ScheduledAt != "" {
			if t, perr := time.Parse(time.RFC3339, it.ScheduledAt); perr == nil && t.Before(cutoff) {
				keep = true
			}
		}
		if keep {
			overdue++
			_ = s.store.SetHoldReason(it.ID, "예정 시각 1일 이상 경과 — 보고 대상으로 유지")
			continue
		}
		if err := s.store.ClearHold(it.ID); err != nil {
			return resumed, overdue, err
		}
		resumed++
	}
	return resumed, overdue, nil
}

// RecordManualPublish records a human-completed publish: the item must be
// queued and scheduled (target set), and the published URL is required as
// evidence. This is the only honest path to published without an executor.
func (s *Service) RecordManualPublish(id int64, url string) (*store.Content, error) {
	c, err := s.store.GetContent(id)
	if err != nil {
		return nil, err
	}
	if c.Status != policy.StatusQueued {
		return nil, errors.New("only queued items can record a publish")
	}
	if strings.TrimSpace(c.TargetAccount) == "" || strings.TrimSpace(c.TargetPlatform) == "" {
		return nil, errors.New("schedule first: target account and platform are required")
	}
	if err := product.ValidateURL(url); err != nil {
		return nil, err
	}
	if err := s.store.UpdateContentStatus(id, policy.StatusPublished); err != nil {
		return nil, err
	}
	doneAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.store.CreatePublishDone(id, c.TargetPlatform, c.TargetAccount,
		doneAt, strings.TrimSpace(url)); err != nil {
		if !errors.Is(err, store.ErrDuplicate) {
			return nil, err
		}
		// A plan record (e.g. auto-advanced) already holds this triplet:
		// complete it instead of failing.
		if _, err := s.store.CompletePublishLog(id, c.TargetPlatform, c.TargetAccount,
			doneAt, strings.TrimSpace(url)); err != nil {
			return nil, err
		}
	}
	return s.store.GetContent(id)
}

// RecordPublishPlan stores a scheduled/held publish-log record. Done/failed
// outcomes need real execution, which is tranche 2, so they are rejected.
func (s *Service) RecordPublishPlan(contentID int64, channel, account, status, note string) (*store.PublishLog, error) {
	if status != "scheduled" && status != "held" {
		return nil, ErrBadPublishLog
	}
	if _, err := s.store.GetContent(contentID); err != nil {
		return nil, err
	}
	return s.store.CreatePublishLog(contentID, channel, account, status, note)
}

// UpdatePublishPlan rewrites a plan record's status and/or note. Status stays
// within scheduled/held: done/failed outcomes need real execution (tranche 2).
func (s *Service) UpdatePublishPlan(id int64, status, note *string) (*store.PublishLog, error) {
	cur, err := s.store.GetPublishLogByID(id)
	if err != nil {
		return nil, err
	}
	if status != nil {
		if *status != "scheduled" && *status != "held" {
			return nil, ErrBadPublishLog
		}
		cur.Status = *status
	}
	if note != nil {
		cur.Note = *note
	}
	if err := s.store.UpdatePublishLog(id, cur.Status, cur.Note); err != nil {
		return nil, err
	}
	return s.store.GetPublishLogByID(id)
}

// MetricInput is a reported performance record.
type MetricInput struct {
	ContentID *int64
	Channel   string
	Exposures *int64
	Inflows   *int64
	Orders    *int64
	Inquiries *int64
	Note      string
}

// RecordMetric stores one record. Zero/missing figures get '원인 미분류' with
// no interpretation attached.
func (s *Service) RecordMetric(in MetricInput) (*store.Metric, error) {
	cause := ""
	if allZeroOrMissing(in.Exposures, in.Inflows, in.Orders, in.Inquiries) {
		cause = policy.UnclassifiedCause
	}
	return s.store.CreateMetric(&store.Metric{
		ContentID: in.ContentID, Channel: in.Channel,
		Exposures: in.Exposures, Inflows: in.Inflows, Orders: in.Orders, Inquiries: in.Inquiries,
		CauseClass: cause, Note: in.Note,
	})
}

func allZeroOrMissing(vals ...*int64) bool {
	for _, v := range vals {
		if v != nil && *v != 0 {
			return false
		}
	}
	return true
}
