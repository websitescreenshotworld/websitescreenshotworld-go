package wsw

import (
	"encoding/json"
	"fmt"
	"time"
)

// Event is a webhook delivery, returned by [VerifyWebhook]. Switch on its concrete type:
//
//	switch e := event.(type) {
//	case *wsw.InstantSnapCaptureSucceededEvent:
//		// e.Data is the capture
//	case *wsw.UnknownEvent:
//		// a type this version does not know; e.Type and e.Data are set
//	}
type Event interface {
	// Envelope returns the fields every delivery carries.
	Envelope() EventEnvelope
}

// EventEnvelope is the part of a delivery common to every event.
type EventEnvelope struct {
	// ID identifies the delivery; retries of the same delivery share it.
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Product   Product   `json:"product"`
}

// Envelope returns e.
func (e EventEnvelope) Envelope() EventEnvelope { return e }

// Event types, as sent in [EventEnvelope.Type].
const (
	EventTypeCaptureSucceeded    = "capture.succeeded"
	EventTypeCaptureFailed       = "capture.failed"
	EventTypeCaptureChanged      = "capture.changed"
	EventTypeDeliveryCompleted   = "delivery.completed"
	EventTypeSchedulePaused      = "schedule.paused"
	EventTypeSitemapRunCompleted = "sitemap_run.completed"
	EventTypePing                = "ping"
)

// InstantSnapCaptureSucceededEvent is sent when an Instant Snap capture with a callback URL
// succeeds. Data has no download URLs; call [InstantSnapService.Get] for them.
type InstantSnapCaptureSucceededEvent struct {
	EventEnvelope
	Data Capture `json:"data"`
}

// InstantSnapCaptureFailedEvent is sent when an Instant Snap capture with a callback URL fails.
// Data.ErrorCode says why.
type InstantSnapCaptureFailedEvent struct {
	EventEnvelope
	Data Capture `json:"data"`
}

// AutoScheduleCaptureSucceededEvent is sent when a scheduled capture succeeds.
type AutoScheduleCaptureSucceededEvent struct {
	EventEnvelope
	Data ScheduledCapture `json:"data"`
}

// AutoScheduleCaptureFailedEvent is sent when a scheduled capture fails.
type AutoScheduleCaptureFailedEvent struct {
	EventEnvelope
	Data ScheduledCapture `json:"data"`
}

// CaptureChangedEvent is sent when a scheduled capture differs from the one before it.
type CaptureChangedEvent struct {
	EventEnvelope
	Data CaptureChange `json:"data"`
}

// DeliveryCompletedEvent is sent when a scheduled capture has been delivered to its
// destinations.
type DeliveryCompletedEvent struct {
	EventEnvelope
	Data Delivery `json:"data"`
}

// SchedulePausedEvent is sent when a schedule stops running, for example because the plan's
// quota is exhausted.
type SchedulePausedEvent struct {
	EventEnvelope
	Data SchedulePause `json:"data"`
}

// SitemapRunCompletedEvent is sent when a run over a sitemap's pages ends.
type SitemapRunCompletedEvent struct {
	EventEnvelope
	Data SitemapRun `json:"data"`
}

// PingEvent is a test delivery sent from the dashboard, by either product.
type PingEvent struct {
	EventEnvelope
	Data Ping `json:"data"`
}

// UnknownEvent is an event whose type this version of the library does not know.
type UnknownEvent struct {
	EventEnvelope
	// Data is the event's data as sent.
	Data json.RawMessage `json:"data"`
}

// ScheduledCapture is an Auto Schedule capture as webhooks describe it.
type ScheduledCapture struct {
	ID string `json:"id"`
	// ScheduleID is the web page's id, the one [AutoScheduleService.TriggerNow] takes.
	ScheduleID     string        `json:"schedule_id"`
	ScheduleName   string        `json:"schedule_name"`
	URL            string        `json:"url"`
	Status         string        `json:"status"`
	MediaCategory  MediaCategory `json:"media_category"`
	Format         Format        `json:"format"`
	PageHTTPStatus int           `json:"page_http_status"`
	CapturedAt     time.Time     `json:"captured_at"`
	FinalURL       string        `json:"final_url"`
	MediaSHA256    string        `json:"media_sha256"`
	// PageSource carries only Format and SHA256, and is nil when no source was kept.
	PageSource *PageSource `json:"page_source"`
	// SitemapRunID is set when the capture was part of a sitemap run.
	SitemapRunID string `json:"sitemap_run_id"`
}

// CaptureChange is a scheduled capture that differs from an earlier one.
type CaptureChange struct {
	ScheduledCapture
	ChangePercent        float64 `json:"change_percent"`
	ComparedToCaptureID  string  `json:"compared_to_capture_id"`
	ChangeImageAvailable bool    `json:"change_image_available"`
}

// Delivery is a scheduled capture and where it was delivered.
type Delivery struct {
	ScheduledCapture
	Destinations []Destination `json:"destinations"`
}

// Destination is one place a scheduled capture was delivered to.
type Destination struct {
	Slot        int       `json:"slot"`
	Name        string    `json:"name"`
	Provider    string    `json:"provider"`
	Status      string    `json:"status"`
	CompletedAt time.Time `json:"completed_at"`
}

// SchedulePause says which schedule stopped and why.
type SchedulePause struct {
	ScheduleID   string    `json:"schedule_id"`
	ScheduleName string    `json:"schedule_name"`
	URL          string    `json:"url"`
	Reason       string    `json:"reason"`
	PausedAt     time.Time `json:"paused_at"`
}

// SitemapRun is the outcome of a run over a sitemap's pages.
type SitemapRun struct {
	SitemapRunID   string    `json:"sitemap_run_id"`
	ScheduleID     string    `json:"schedule_id"`
	ScheduleName   string    `json:"schedule_name"`
	SitemapURL     string    `json:"sitemap_url"`
	Status         string    `json:"status"`
	RunAt          time.Time `json:"run_at"`
	CompletedAt    time.Time `json:"completed_at"`
	PagesListed    int       `json:"pages_listed"`
	PagesMatched   int       `json:"pages_matched"`
	PagesQueued    int       `json:"pages_queued"`
	PagesSucceeded int       `json:"pages_succeeded"`
	PagesFailed    int       `json:"pages_failed"`
	PagesOverLimit int       `json:"pages_over_limit"`
	PagesOverQuota int       `json:"pages_over_quota"`
	Message        string    `json:"message"`
}

// Ping is the data of a test delivery.
type Ping struct {
	EndpointID   string    `json:"endpoint_id"`
	EndpointName string    `json:"endpoint_name"`
	Message      string    `json:"message"`
	SentAt       time.Time `json:"sent_at"`
}

// parseEvent decodes a delivery into the event type its type and product name.
func parseEvent(body []byte) (Event, error) {
	var env EventEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("wsw: decoding webhook: %w", err)
	}

	var e Event
	switch {
	case env.Type == EventTypeCaptureSucceeded && env.Product == ProductInstantSnap:
		e = &InstantSnapCaptureSucceededEvent{}
	case env.Type == EventTypeCaptureFailed && env.Product == ProductInstantSnap:
		e = &InstantSnapCaptureFailedEvent{}
	case env.Type == EventTypeCaptureSucceeded && env.Product == ProductAutoSchedule:
		e = &AutoScheduleCaptureSucceededEvent{}
	case env.Type == EventTypeCaptureFailed && env.Product == ProductAutoSchedule:
		e = &AutoScheduleCaptureFailedEvent{}
	case env.Type == EventTypeCaptureChanged:
		e = &CaptureChangedEvent{}
	case env.Type == EventTypeDeliveryCompleted:
		e = &DeliveryCompletedEvent{}
	case env.Type == EventTypeSchedulePaused:
		e = &SchedulePausedEvent{}
	case env.Type == EventTypeSitemapRunCompleted:
		e = &SitemapRunCompletedEvent{}
	case env.Type == EventTypePing:
		e = &PingEvent{}
	default:
		e = &UnknownEvent{}
	}
	if err := json.Unmarshal(body, e); err != nil {
		return nil, fmt.Errorf("wsw: decoding %s webhook: %w", env.Type, err)
	}
	return e, nil
}
