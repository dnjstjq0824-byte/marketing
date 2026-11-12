// Package queue implements the worker queue spec v1 (§5) verbatim.
package queue

import (
	"encoding/json"

	"somun/internal/store"
)

// Job types: exactly analyze and generate. Never add publish/check/metric jobs.
const (
	JobAnalyze  = "analyze"
	JobGenerate = "generate"
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

// GenerateItem is one requested content draft.
type GenerateItem struct {
	Channel        string `json:"channel"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	VirtualPersona bool   `json:"virtual_persona"`
}

// GenerateResult is the worker's content-generation payload.
type GenerateResult struct {
	Contents []GenerateItem `json:"contents"`
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
}

func toView(j *store.Job) *JobView {
	return &JobView{
		ID: j.ID, Type: j.Type, RefID: j.RefID, Status: j.Status, WorkerName: j.WorkerName,
		Payload: rawMessage(j.Payload), CreatedAt: j.CreatedAt, ClaimedAt: j.ClaimedAt,
		FinishedAt: j.FinishedAt, Error: j.Error,
	}
}

func rawMessage(s string) any {
	if s == "" {
		return map[string]any{}
	}
	return json.RawMessage(s)
}
