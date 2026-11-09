package wsw

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// AutoScheduleService starts the capture of a web page scheduled in the dashboard. Use it
// through [Client.AutoSchedule]. Results arrive by webhook.
type AutoScheduleService struct {
	client *Client
}

// TriggerResult is the answer of [AutoScheduleService.TriggerNow].
type TriggerResult struct {
	Status      TriggerStatus `json:"status"`
	WebPageID   string        `json:"web_page_id"`
	TriggeredBy TriggeredBy   `json:"triggered_by"`
	// TriggeredAt is when the capture was queued; for an already pending capture, when it was
	// first triggered.
	TriggeredAt time.Time `json:"triggered_at"`
	// Message is one sentence for a person.
	Message string `json:"message"`
}

// TriggerNow queues a capture of the web page now. webPageID is the id shown in the dashboard,
// which webhooks call schedule_id. It is idempotent: while a capture of the page is pending,
// it answers [TriggerStatusAlreadyPending] and starts nothing.
func (s *AutoScheduleService) TriggerNow(ctx context.Context, webPageID string) (*TriggerResult, error) {
	if s.client.autoScheduleKey == "" {
		return nil, fmt.Errorf("%w: the Auto Schedule key is not set; use wsw.WithAutoScheduleKey or set %s", ErrMissingKey, EnvAutoScheduleKey)
	}
	if webPageID == "" {
		return nil, errors.New("wsw: web page id is empty")
	}
	var r TriggerResult
	_, err := s.client.sendJSON(ctx, &request{
		method:  http.MethodPost,
		url:     s.client.autoScheduleBaseURL + "/auto-schedule/v1/web-pages/" + url.PathEscape(webPageID) + "/trigger-now",
		apiKey:  s.client.autoScheduleKey,
		timeout: s.client.timeout,
	}, &r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
