// Package queue implements the worker queue spec v1 (§5) verbatim.
package queue

import (
	"encoding/json"

	"somun/internal/store"
)

// Job types: analyze, strategy, generate, review, publish.
// Chain: analyze -> strategy -> generate -> review (A, B, then C chief);
// publish (P lane) carries session posts in P1-2.
const (
	JobAnalyze  = "analyze"
	JobStrategy = "strategy"
	JobGenerate = "generate"
	JobReview   = "review"
	JobPublish  = "publish"
	// JobResearch reads a topic-only input (W1): outline + key points
	// instead of page parsing. It feeds the strategy job like analyze.
	JobResearch = "research"
	// JobMedia generates images/video for one post (W14). It is a leaf:
	// failure never blocks the text chain.
	JobMedia = "media"
	// JobCollect scrapes one published URL's public figures (W7). A leaf
	// on the P lane: failure never touches the text pipeline.
	JobCollect = "collect"
)

// Job statuses: pending -> claimed -> done/failed.
const (
	StatusPending = "pending"
	StatusClaimed = "claimed"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

// DefaultWorkerName is recorded when X-Worker-Name is absent.
const DefaultWorkerName = "resident-claude"

// EmptyQueueMessage is the Korean message for "no waiting job".
const EmptyQueueMessage = "대기 중인 작업이 없습니다"

// AnalyzeResult is the worker's product-reading payload: 8 strings (§5).
// Unconfirmed items stay empty or go into Unknowns; the worker never invents.
type AnalyzeResult struct {
	ProductName   string `json:"product_name"`
	PriceSummary  string `json:"price_summary"`
	OptionsText   string `json:"options_text"`
	ShippingText  string `json:"shipping_text"`
	DetailCore    string `json:"detail_core"`
	ReviewSummary string `json:"review_summary"`
	ImageDesc     string `json:"image_desc"`
	Unknowns      string `json:"unknowns"`
}

// ResearchResult is the worker's topic-study payload (W1): the topic
// title, key points, and a post outline. It becomes the work's reading.
type ResearchResult struct {
	TopicTitle string `json:"topic_title"`
	KeyPoints  string `json:"key_points"`
	Outline    string `json:"outline"`
	Unknowns   string `json:"unknowns"`
}

// StrategyResult is the worker's marketing-strategy payload: 4 strings.
// Angle is required; the rest may stay empty.
type StrategyResult struct {
	Angle        string `json:"angle"`
	Channels     string `json:"channels"`
	KeyMessages  string `json:"key_messages"`
	ScheduleNote string `json:"schedule_note"`
}

// GenerateItem is one requested content draft.
type GenerateItem struct {
	Channel        string `json:"channel"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	VirtualPersona bool   `json:"virtual_persona"`
	// Platform optionally pins the brief entry (W3-2); empty keeps the
	// legacy first-match resolution.
	Platform string `json:"platform"`
	// Media is the worker's media shopping list (W3-4), stored verbatim.
	Media string `json:"media"`
}

// GenerateResult is the worker's content-generation payload.
type GenerateResult struct {
	Contents []GenerateItem `json:"contents"`
}

// ReviewDecisionItem is one chief judgment on a draft.
// W4: Account/Platform/ScheduledAt are legacy worker echoes and are ignored;
// the server scheduler assigns on approval.
type ReviewDecisionItem struct {
	ContentID   int64  `json:"content_id"`
	Approve     bool   `json:"approve"`
	Reason      string `json:"reason"`
	Account     string `json:"account"`
	Platform    string `json:"platform"`
	ScheduledAt string `json:"scheduledAt"`
}

// ReviewResult is the chief worker's review payload.
type ReviewResult struct {
	Decisions []ReviewDecisionItem `json:"decisions"`
}

// MediaResultItem is one finished file from a media job.
type MediaResultItem struct {
	MediaID int64  `json:"media_id"`
	Role    string `json:"role"`
}

// MediaResult is the media worker's payload: finished file ids plus how
// they were made. Empty MediaIDs with a Note records a graceful miss
// (the post falls back to original images); hard failures use /fail.
type MediaResult struct {
	MediaIDs []MediaResultItem `json:"media_ids"`
	Note     string            `json:"note"`
	Fallback string            `json:"fallback"`
}

// PublishResult is the adapter's session-post report.
type PublishResult struct {
	URL      string `json:"url"`
	Evidence string `json:"evidence"`
}

// CollectResult is the P worker's performance report for one window.
// Public figures (Views/Likes/Comments) come from the open post page;
// the funnel (Exposures..Inquiries) from the operator stats screen.
// Nil means "not collected": the row keeps cause '원인 미분류'.
type CollectResult struct {
	Views     *int64 `json:"views"`
	Likes     *int64 `json:"likes"`
	Comments  *int64 `json:"comments"`
	Exposures *int64 `json:"exposures"`
	Inflows   *int64 `json:"inflows"`
	Orders    *int64 `json:"orders"`
	Inquiries *int64 `json:"inquiries"`
	Note      string `json:"note"`
}

// AllowedChannels are the only 4 generate channels. Anything else is dropped.
var AllowedChannels = map[string]bool{
	"instagram": true,
	"shortform": true,
	"cardnews":  true,
	"blog":      true,
}

// JobView is the API shape of a job.
type JobView struct {
	ID         int64  `json:"id"`
	Type       string `json:"type"`
	RefID      int64  `json:"refId"`
	Status     string `json:"status"`
	WorkerName string `json:"workerName"`
	Payload    any    `json:"payload"`
	CreatedAt  string `json:"createdAt"`
	ClaimedAt  string `json:"claimedAt"`
	FinishedAt string `json:"finishedAt"`
	Error      string `json:"error"`
	AgentID    int64  `json:"agentId"`
	Business   string `json:"business"`
	Priority   int    `json:"priority"`
	Attempts   int    `json:"attempts"`
	LeaseUntil string `json:"leaseUntil"`
}

func toView(j *store.Job) *JobView {
	return &JobView{
		ID: j.ID, Type: j.Type, RefID: j.RefID, Status: j.Status, WorkerName: j.WorkerName,
		Payload: rawMessage(j.Payload), CreatedAt: j.CreatedAt, ClaimedAt: j.ClaimedAt,
		FinishedAt: j.FinishedAt, Error: j.Error, AgentID: j.AgentID, Business: j.Business,
		Priority: j.Priority, Attempts: j.Attempts, LeaseUntil: j.LeaseUntil,
	}
}

func rawMessage(s string) any {
	if s == "" {
		return map[string]any{}
	}
	return json.RawMessage(s)
}
