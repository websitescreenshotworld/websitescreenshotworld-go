package wsw_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/websitescreenshotworld/websitescreenshotworld-go"
)

type webhookVectors struct {
	Secret string `json:"secret"`
	Cases  []struct {
		Name      string `json:"name"`
		Header    string `json:"header"`
		Body      string `json:"body"`
		Now       int64  `json:"now"`
		Tolerance int64  `json:"tolerance"`
		Valid     bool   `json:"valid"`
	} `json:"cases"`
}

func TestVerifyWebhookVectors(t *testing.T) {
	var vectors webhookVectors
	readJSON(t, "spec/webhook-vectors.json", &vectors)
	if len(vectors.Cases) == 0 {
		t.Fatal("no cases")
	}

	for _, tc := range vectors.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			opts := []wsw.VerifyOption{wsw.WithNow(fixedClock(tc.Now))}
			if tc.Tolerance > 0 {
				opts = append(opts, wsw.WithTolerance(time.Duration(tc.Tolerance)*time.Second))
			}
			event, err := wsw.VerifyWebhook([]byte(tc.Body), tc.Header, vectors.Secret, opts...)

			if !tc.Valid {
				if !errors.Is(err, wsw.ErrSignatureVerification) {
					t.Fatalf("got %v, want a signature verification error", err)
				}
				var apiErr *wsw.Error
				if !errors.As(err, &apiErr) || apiErr.Kind != wsw.KindSignatureVerification {
					t.Fatalf("got %#v, want a *wsw.Error of kind %q", err, wsw.KindSignatureVerification)
				}
				return
			}
			if err != nil {
				t.Fatalf("got %v, want the event", err)
			}
			if _, ok := event.(*wsw.PingEvent); !ok {
				t.Fatalf("got %T, want *wsw.PingEvent", event)
			}
		})
	}
}

func TestVerifyWebhookEmptySecret(t *testing.T) {
	_, err := wsw.VerifyWebhook([]byte("{}"), "t=1,v1=00", "")
	if !errors.Is(err, wsw.ErrSignatureVerification) {
		t.Fatalf("got %v, want a signature verification error", err)
	}
}

func TestWebhookEvents(t *testing.T) {
	checks := map[string]func(t *testing.T, e wsw.Event){
		"auto-schedule.capture.changed.json": func(t *testing.T, e wsw.Event) {
			ev := as[*wsw.CaptureChangedEvent](t, e)
			if ev.Data.ChangePercent != 12.5 || !ev.Data.ChangeImageAvailable || ev.Data.ScheduleID == "" {
				t.Errorf("data = %+v", ev.Data)
			}
		},
		"auto-schedule.capture.failed.json": func(t *testing.T, e wsw.Event) {
			ev := as[*wsw.AutoScheduleCaptureFailedEvent](t, e)
			if ev.Data.Status != "failed" || ev.Data.PageHTTPStatus != 503 || ev.Data.PageSource != nil {
				t.Errorf("data = %+v", ev.Data)
			}
		},
		"auto-schedule.capture.succeeded.json": func(t *testing.T, e wsw.Event) {
			ev := as[*wsw.AutoScheduleCaptureSucceededEvent](t, e)
			if ev.Data.PageSource == nil || ev.Data.PageSource.Format != wsw.PageSourceFormatHTML || ev.Data.Format != wsw.FormatPNG {
				t.Errorf("data = %+v", ev.Data)
			}
			if want := time.Date(2026, 9, 26, 10, 30, 4, 512e6, time.UTC); !ev.Data.CapturedAt.Equal(want) {
				t.Errorf("CapturedAt = %v, want %v", ev.Data.CapturedAt, want)
			}
		},
		"auto-schedule.delivery.completed.json": func(t *testing.T, e wsw.Event) {
			ev := as[*wsw.DeliveryCompletedEvent](t, e)
			if len(ev.Data.Destinations) != 1 || ev.Data.Destinations[0].Provider != "aws_s3" || ev.Data.Status != "media_delivered" {
				t.Errorf("data = %+v", ev.Data)
			}
		},
		"auto-schedule.ping.json": func(t *testing.T, e wsw.Event) {
			ev := as[*wsw.PingEvent](t, e)
			if ev.Product != wsw.ProductAutoSchedule || ev.Data.EndpointName != "Production" {
				t.Errorf("event = %+v", ev)
			}
		},
		"auto-schedule.schedule.paused.json": func(t *testing.T, e wsw.Event) {
			ev := as[*wsw.SchedulePausedEvent](t, e)
			if ev.Data.Reason != "quota_exhausted" || ev.Data.PausedAt.IsZero() {
				t.Errorf("data = %+v", ev.Data)
			}
		},
		"auto-schedule.sitemap_run.completed.json": func(t *testing.T, e wsw.Event) {
			ev := as[*wsw.SitemapRunCompletedEvent](t, e)
			if ev.Data.PagesListed != 120 || ev.Data.PagesOverLimit != 30 || ev.Data.Message != "" {
				t.Errorf("data = %+v", ev.Data)
			}
		},
		"instant-snap.capture.failed.json": func(t *testing.T, e wsw.Event) {
			ev := as[*wsw.InstantSnapCaptureFailedEvent](t, e)
			if ev.Data.ErrorCode != "capture_failed" || ev.Data.Status != wsw.CaptureStatusFailed {
				t.Errorf("data = %+v", ev.Data)
			}
		},
		"instant-snap.capture.succeeded.json": func(t *testing.T, e wsw.Event) {
			ev := as[*wsw.InstantSnapCaptureSucceededEvent](t, e)
			if ev.Data.ID == "" || ev.Data.SizeBytes != 482113 || ev.Data.DownloadURL != "" {
				t.Errorf("data = %+v", ev.Data)
			}
		},
		"unknown.json": func(t *testing.T, e wsw.Event) {
			ev := as[*wsw.UnknownEvent](t, e)
			if ev.Type != "capture.teleported" {
				t.Errorf("Type = %q", ev.Type)
			}
			var data map[string]any
			if err := json.Unmarshal(ev.Data, &data); err != nil || data["anything"] != 1.0 {
				t.Errorf("Data = %s", ev.Data)
			}
		},
	}

	var vectors webhookVectors
	readJSON(t, "spec/webhook-vectors.json", &vectors)
	files, err := filepath.Glob("spec/events/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(checks) {
		t.Errorf("%d event examples, %d checks", len(files), len(checks))
	}

	const now = 1790418600
	for _, file := range files {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			check, ok := checks[name]
			if !ok {
				t.Fatal("no check for this example")
			}
			body, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			header := sign(vectors.Secret, now, body)
			event, err := wsw.VerifyWebhook(body, header, vectors.Secret, wsw.WithNow(fixedClock(now)))
			if err != nil {
				t.Fatal(err)
			}
			env := event.Envelope()
			if env.ID == "" || env.Type == "" || env.CreatedAt.IsZero() {
				t.Errorf("envelope = %+v", env)
			}
			check(t, event)
		})
	}
}

func as[T wsw.Event](t *testing.T, e wsw.Event) T {
	t.Helper()
	v, ok := e.(T)
	if !ok {
		var want T
		t.Fatalf("got %T, want %T", e, want)
	}
	return v
}

func sign(secret string, unix int64, body []byte) string {
	ts := strconv.FormatInt(unix, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	return "t=" + ts + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

func fixedClock(unix int64) func() time.Time {
	return func() time.Time { return time.Unix(unix, 0) }
}
