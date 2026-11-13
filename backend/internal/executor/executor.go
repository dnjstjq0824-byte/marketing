// Package executor runs verdict-ready official-API posts (D28).
package executor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"somun/internal/publisher"
	"somun/internal/session"
	"somun/internal/store"
)

// MaxAttempts caps tries per record before it closes as failed.
const MaxAttempts = 5

// Service executes due official-API items sequentially, at most maxActive
// per tick. (True parallelism waits for real blocking publishers in P1-2;
// sequential trivially honors the concurrency cap.)
type Service struct {
	store      *store.Store
	session    *session.Service
	publishers map[string]publisher.Publisher
	defaults   store.ExecutorConfig
	// OnPublishFailed fires on final failures and limited flips (notify hook).
	OnPublishFailed func(contentID int64, message string)
	// OnLimitedFlip fires when a mid-flight limit flips an account.
	OnLimitedFlip func(accountID int64, name string)
}

// NewService builds the executor.
func NewService(st *store.Store, sess *session.Service, pubs map[string]publisher.Publisher, defaults store.ExecutorConfig) *Service {
	return &Service{store: st, session: sess, publishers: pubs, defaults: defaults}
}

// Config returns the effective triplet (app_meta over ENV defaults).
func (s *Service) Config() (store.ExecutorConfig, error) {
	return store.GetExecutorConfig(s.store.DB(), s.defaults.HeartbeatMin, s.defaults.MinIntervalMin, s.defaults.MaxActive)
}

// SetConfig validates and persists the triplet. Interval clamps to 0..1440.
func (s *Service) SetConfig(cfg store.ExecutorConfig) (store.ExecutorConfig, error) {
	if cfg.HeartbeatMin < 0 || cfg.MaxActive < 1 {
		return cfg, fmt.Errorf("heartbeatMin must be >= 0 and maxActive >= 1")
	}
	if cfg.MinIntervalMin < 0 {
		cfg.MinIntervalMin = 0
	}
	if cfg.MinIntervalMin > 1440 {
		cfg.MinIntervalMin = 1440
	}
	if err := store.SetExecutorConfig(s.store.DB(), cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Tick executes due items once. Heartbeat 0 stops execution (verdicts only).
func (s *Service) Tick(now time.Time) (int, error) {
	cfg, err := s.Config()
	if err != nil {
		return 0, err
	}
	if cfg.HeartbeatMin <= 0 {
		return 0, nil
	}
	if cfg.MaxActive < 1 {
		cfg.MaxActive = 1
	}
	nowUTC := now.UTC()
	plan := s.session.PublishPlan(nowUTC)
	done := 0
	for _, it := range plan {
		if done >= cfg.MaxActive {
			break
		}
		if it.Group != "official_api" || it.Verdict != session.VerdictReady {
			continue
		}
		if err := s.executeOne(it, cfg, nowUTC); err != nil {
			return done, err
		}
		done++
	}
	return done, nil
}

func (s *Service) executeOne(it *session.PlanItem, cfg store.ExecutorConfig, now time.Time) error {
	content, err := s.store.GetContent(it.ContentID)
	if err != nil {
		return err
	}
	channel := content.TargetPlatform
	if channel == "" {
		channel = content.Channel
	}
	rec, err := s.store.GetPublishLog(content.ID, channel, it.Account)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		rec, err = s.store.CreatePublishLog(content.ID, channel, it.Account, "scheduled", "")
		if err != nil {
			return err
		}
	}
	if rec.Status != "scheduled" || rec.Attempts >= MaxAttempts {
		return nil
	}
	if rec.NextEligibleAt != "" {
		if eligible, err := time.Parse(time.RFC3339, rec.NextEligibleAt); err != nil || eligible.After(now) {
			return nil
		}
	}
	// Same-account spacing (D29): skip until the interval passes.
	if last, err := s.store.LastDoneAt(it.Account); err != nil {
		return err
	} else if last != "" {
		if doneAt, err := time.Parse(time.RFC3339, last); err == nil {
			next := doneAt.Add(time.Duration(cfg.MinIntervalMin) * time.Minute)
			if next.After(now) {
				_ = s.store.MarkLogAttempt(rec.ID, rec.Attempts, "min_interval gate", next.Format(time.RFC3339))
				return nil
			}
		}
	}
	pub, ok := s.publishers[channel]
	if !ok {
		return s.failAttempt(rec, content.ID, fmt.Sprintf("no publisher for %s", channel), cfg, now)
	}
	acc, err := s.store.GetAccountRowByName(it.Account)
	if err != nil {
		return s.failAttempt(rec, content.ID, "account vanished mid-flight", cfg, now)
	}
	loginID, password, err := s.session.CredentialsFor(acc.ID)
	if err != nil {
		return s.failAttempt(rec, content.ID, "credentials unavailable", cfg, now)
	}
	res, err := pub.Publish(context.Background(), publisher.Request{
		ContentID: content.ID, Channel: channel, Title: content.Title, Body: content.Body,
		Account: it.Account, LoginID: loginID, Password: password,
	})
	if err != nil {
		if errors.Is(err, publisher.ErrSessionLimited) {
			return s.limitedFlip(rec, acc, content.ID, now)
		}
		return s.failAttempt(rec, content.ID, err.Error(), cfg, now)
	}
	if err := s.store.MarkLogDone(rec.ID, now.Format(time.RFC3339), res.URL); err != nil {
		return err
	}
	return s.store.MarkContentPublished(content.ID)
}

func (s *Service) failAttempt(rec *store.PublishLog, contentID int64, reason string, cfg store.ExecutorConfig, now time.Time) error {
	attempts := rec.Attempts + 1
	if attempts >= MaxAttempts {
		if err := s.store.MarkLogFailed(rec.ID, attempts, reason); err != nil {
			return err
		}
		if s.OnPublishFailed != nil {
			s.OnPublishFailed(contentID, fmt.Sprintf("콘텐츠 #%d: 발행 %d회 실패 — %s", contentID, attempts, reason))
		}
		return nil
	}
	next := now.Add(time.Duration(cfg.MinIntervalMin) * time.Minute).Format(time.RFC3339)
	return s.store.MarkLogAttempt(rec.ID, attempts, reason, next)
}

func (s *Service) limitedFlip(rec *store.PublishLog, acc *store.ChannelAccount, contentID int64, now time.Time) error {
	_ = now
	if err := s.store.MarkLogFailed(rec.ID, rec.Attempts+1, "session limited mid-flight"); err != nil {
		return err
	}
	if _, err := s.session.SetSessionState(acc.ID, session.StateLimited, "executor"); err != nil {
		return err
	}
	if s.OnLimitedFlip != nil {
		s.OnLimitedFlip(acc.ID, acc.Name)
	}
	if s.OnPublishFailed != nil {
		s.OnPublishFailed(contentID, fmt.Sprintf("콘텐츠 #%d: 실행 중 세션 한도 — 계정 %s 보류", contentID, acc.Name))
	}
	return nil
}
