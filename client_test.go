package wsw_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/websitescreenshotworld/websitescreenshotworld-go"
)

// newServer starts a test server and returns a client pointed at it for both products.
func newServer(t *testing.T, h http.HandlerFunc, opts ...wsw.ClientOption) (*httptest.Server, *wsw.Client) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	opts = append([]wsw.ClientOption{
		wsw.WithInstantSnapKey("is-key"),
		wsw.WithAutoScheduleKey("as-key"),
		wsw.WithBaseURLs(srv.URL, srv.URL),
	}, opts...)
	return srv, wsw.NewClient(opts...)
}

func TestMissingKeyFailsBeforeSending(t *testing.T) {
	t.Setenv(wsw.EnvInstantSnapKey, "")
	t.Setenv(wsw.EnvAutoScheduleKey, "")
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer srv.Close()
	client := wsw.NewClient(wsw.WithBaseURLs(srv.URL, srv.URL))
	ctx := context.Background()

	_, err := client.InstantSnap.Capture(ctx, &wsw.CaptureOptions{URL: "https://example.com"})
	if !errors.Is(err, wsw.ErrMissingKey) || !strings.Contains(err.Error(), wsw.EnvInstantSnapKey) {
		t.Errorf("Capture: got %v, want a missing Instant Snap key error", err)
	}
	_, err = client.InstantSnap.Get(ctx, "id")
	if !errors.Is(err, wsw.ErrMissingKey) {
		t.Errorf("Get: got %v, want a missing key error", err)
	}
	_, err = client.AutoSchedule.TriggerNow(ctx, "id")
	if !errors.Is(err, wsw.ErrMissingKey) || !strings.Contains(err.Error(), wsw.EnvAutoScheduleKey) {
		t.Errorf("TriggerNow: got %v, want a missing Auto Schedule key error", err)
	}
	if n := calls.Load(); n != 0 {
		t.Errorf("%d requests were sent", n)
	}
}

func TestKeysDefaultFromEnvironment(t *testing.T) {
	t.Setenv(wsw.EnvAutoScheduleKey, "from-env")
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Api-Key")
		io.WriteString(w, `{"success":true,"status":"triggered"}`)
	}))
	defer srv.Close()

	client := wsw.NewClient(wsw.WithBaseURLs("", srv.URL))
	if _, err := client.AutoSchedule.TriggerNow(context.Background(), "page"); err != nil {
		t.Fatal(err)
	}
	if got != "from-env" {
		t.Errorf("X-Api-Key = %q, want the environment's key", got)
	}
}

func TestRequestHeaders(t *testing.T) {
	uaPattern := regexp.MustCompile(`^websitescreenshotworld-go/` + regexp.QuoteMeta(wsw.Version) + ` \(go [^ ()]+\)$`)
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	headers := map[string]http.Header{}
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		headers[r.URL.Path] = r.Header.Clone()
		switch r.URL.Path {
		case "/instant-snap/v1/defaults":
			io.WriteString(w, `{"success":true,"spec_version":"v1"}`)
		default:
			w.Header().Set("Content-Type", "image/png")
			io.WriteString(w, "png")
		}
	})
	ctx := context.Background()

	if _, err := client.InstantSnap.Defaults(ctx); err != nil {
		t.Fatal(err)
	}
	file, err := client.InstantSnap.CaptureFile(ctx, &wsw.CaptureOptions{URL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	file.Close()

	defaults := headers["/instant-snap/v1/defaults"]
	if ua := defaults.Get("User-Agent"); !uaPattern.MatchString(ua) {
		t.Errorf("User-Agent = %q", ua)
	}
	if defaults.Get("X-Api-Key") != "" {
		t.Error("defaults sent an API key")
	}
	if defaults.Get("Accept") != "application/json" {
		t.Errorf("defaults Accept = %q", defaults.Get("Accept"))
	}
	if defaults.Get("Content-Type") != "" {
		t.Errorf("defaults Content-Type = %q, want none without a body", defaults.Get("Content-Type"))
	}

	capture := headers["/instant-snap/v1/captures"]
	if capture.Get("Accept") == "application/json" {
		t.Error("CaptureFile asked for JSON")
	}
	if key := capture.Get("Idempotency-Key"); !uuidPattern.MatchString(key) {
		t.Errorf("Idempotency-Key = %q, want a random UUID", key)
	}
	if capture.Get("X-Api-Key") != "is-key" {
		t.Errorf("X-Api-Key = %q", capture.Get("X-Api-Key"))
	}
}

func TestIdempotencyKeyIsNewPerCall(t *testing.T) {
	var keys []string
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		io.WriteString(w, `{"success":true}`)
	})
	for range 2 {
		if _, err := client.InstantSnap.Capture(context.Background(), &wsw.CaptureOptions{URL: "https://example.com"}); err != nil {
			t.Fatal(err)
		}
	}
	if keys[0] == keys[1] {
		t.Errorf("both calls sent %q", keys[0])
	}
}

func TestCaptureReplayed(t *testing.T) {
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Idempotent-Replayed", "true")
		io.WriteString(w, `{"success":true,"id":"a"}`)
	})
	c, err := client.InstantSnap.Capture(context.Background(), &wsw.CaptureOptions{URL: "https://example.com"}, wsw.WithIdempotencyKey("k"))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Replayed {
		t.Error("Replayed = false")
	}
}

func TestCaptureNilOptions(t *testing.T) {
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) { t.Error("request sent") })
	if _, err := client.InstantSnap.Capture(context.Background(), nil); err == nil {
		t.Error("no error for nil options")
	}
}

func TestDownload(t *testing.T) {
	var mediaHeaders http.Header
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mediaHeaders = r.Header.Clone()
		if r.URL.Path != "/file.png" {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, "file bytes")
	}))
	defer media.Close()
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {})
	ctx := context.Background()

	var buf bytes.Buffer
	n, err := client.InstantSnap.Download(ctx, &wsw.Capture{DownloadURL: media.URL + "/file.png"}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 10 || buf.String() != "file bytes" {
		t.Errorf("wrote %d bytes %q", n, buf.String())
	}
	if mediaHeaders.Get("X-Api-Key") != "" {
		t.Error("the API key was sent to the download host")
	}

	path := filepath.Join(t.TempDir(), "capture.png")
	if err := client.InstantSnap.DownloadToFile(ctx, &wsw.Capture{DownloadURL: media.URL + "/file.png"}, path); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != "file bytes" {
		t.Errorf("file holds %q", data)
	}

	err = client.InstantSnap.DownloadToFile(ctx, &wsw.Capture{DownloadURL: media.URL + "/gone.png"}, path)
	if !errors.Is(err, wsw.ErrNotFound) {
		t.Errorf("got %v, want a not found error", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "file bytes" {
		t.Errorf("a failed download changed the existing file to %q", data)
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("%d files left in the folder, want 1", len(entries))
	}

	if _, err := client.InstantSnap.Download(ctx, &wsw.Capture{}, &buf); err == nil {
		t.Error("no error for a capture without a download URL")
	}
}

func TestCaptureFileSaveTo(t *testing.T) {
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''r%C3%A9sum%C3%A9.pdf`)
		w.Header().Set("X-InstantSnap-Captured-At", "2026-09-26T10:30:04.5+05:30")
		w.Header().Set("X-InstantSnap-Size-Bytes", "3")
		io.WriteString(w, "pdf")
	})
	f, err := client.InstantSnap.CaptureFile(context.Background(), &wsw.CaptureOptions{URL: "https://example.com", Format: wsw.FormatDocumentPDF})
	if err != nil {
		t.Fatal(err)
	}
	if f.Filename != "résumé.pdf" {
		t.Errorf("Filename = %q", f.Filename)
	}
	if want := time.Date(2026, 9, 26, 5, 0, 4, 500e6, time.UTC); !f.CapturedAt.Equal(want) {
		t.Errorf("CapturedAt = %v, want %v", f.CapturedAt, want)
	}
	if f.SizeBytes != 3 || f.PartCount != 1 {
		t.Errorf("SizeBytes = %d, PartCount = %d", f.SizeBytes, f.PartCount)
	}

	path := filepath.Join(t.TempDir(), f.Filename)
	if err := f.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != "pdf" {
		t.Errorf("file holds %q", data)
	}
}

func TestDatesParseLeniently(t *testing.T) {
	cases := map[string]time.Time{
		"2026-09-26T10:30:04Z":                time.Date(2026, 9, 26, 10, 30, 4, 0, time.UTC),
		"2026-09-26T10:30:04.5Z":              time.Date(2026, 9, 26, 10, 30, 4, 500e6, time.UTC),
		"2026-09-26T10:30:04.512Z":            time.Date(2026, 9, 26, 10, 30, 4, 512e6, time.UTC),
		"2026-09-26T10:30:04.123456789Z":      time.Date(2026, 9, 26, 10, 30, 4, 123456789, time.UTC),
		"2026-09-26T12:30:04.512+02:00":       time.Date(2026, 9, 26, 10, 30, 4, 512e6, time.UTC),
		"2026-09-26T10:30:04.512000000-00:00": time.Date(2026, 9, 26, 10, 30, 4, 512e6, time.UTC),
	}
	for in, want := range cases {
		var c wsw.Capture
		body := `{"id":"a","captured_at":"` + in + `","expires_at":null,"added_later":{"x":1}}`
		if err := json.Unmarshal([]byte(body), &c); err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		if !c.CapturedAt.Equal(want) {
			t.Errorf("%s parsed as %v, want %v", in, c.CapturedAt, want)
		}
		if !c.ExpiresAt.IsZero() {
			t.Errorf("null expires_at parsed as %v", c.ExpiresAt)
		}
	}
}

func TestConnectionFailureIsRetriedAndWrapped(t *testing.T) {
	var calls atomic.Int32
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			conn.Close()
		}
	}, wsw.WithMaxRetries(1))

	_, err := client.InstantSnap.Get(context.Background(), "id")
	if !errors.Is(err, wsw.ErrConnection) {
		t.Fatalf("got %v, want a connection error", err)
	}
	var apiErr *wsw.Error
	if !errors.As(err, &apiErr) || apiErr.Kind != wsw.KindConnection || errors.Unwrap(err) == nil {
		t.Errorf("got %#v, want a connection *wsw.Error wrapping the cause", err)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("%d attempts, want 2", n)
	}
}

func TestTimeoutEndsTheCall(t *testing.T) {
	var calls atomic.Int32
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}, wsw.WithTimeout(100*time.Millisecond))

	start := time.Now()
	_, err := client.InstantSnap.Catalog(context.Background())
	if !errors.Is(err, wsw.ErrConnection) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want a connection error matching context.DeadlineExceeded", err)
	}
	if !strings.Contains(err.Error(), "no answer within 100ms") {
		t.Errorf("got %q, want it to name the timeout", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("returned after %v, want about 100ms", elapsed)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("%d attempts, want 1: a timed-out call is not retried", n)
	}
}

func TestTimeoutCoversRetries(t *testing.T) {
	var calls atomic.Int32
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(w, `{"success":false,"error_code":"service_unavailable","error_message":"Busy.","retry_after_seconds":0.05}`)
	}, wsw.WithTimeout(300*time.Millisecond), wsw.WithMaxRetries(1000))

	start := time.Now()
	_, err := client.InstantSnap.Catalog(context.Background())
	if err == nil {
		t.Fatal("got no error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("returned after %v, want about 300ms", elapsed)
	}
	if n := calls.Load(); n < 2 || n > 10 {
		t.Errorf("%d attempts, want a few retries within the 300ms", n)
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	for _, custom := range []bool{false, true} {
		var elsewhere atomic.Int32
		other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			elsewhere.Add(1)
		}))
		defer other.Close()
		opts := []wsw.ClientOption{}
		if custom {
			opts = append(opts, wsw.WithHTTPClient(&http.Client{}))
		}
		_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, other.URL+"/steal", http.StatusFound)
		}, opts...)

		_, err := client.InstantSnap.Get(context.Background(), "id")
		var apiErr *wsw.Error
		if !errors.As(err, &apiErr) || apiErr.HTTPStatus != http.StatusFound {
			t.Fatalf("custom client %v: got %v, want the 302 as an error", custom, err)
		}
		if n := elsewhere.Load(); n != 0 {
			t.Errorf("custom client %v: the redirect was followed %d times", custom, n)
		}
	}
}

func TestOwnHTTPClientIsNotChanged(t *testing.T) {
	hc := &http.Client{}
	wsw.NewClient(wsw.WithHTTPClient(hc))
	if hc.CheckRedirect != nil {
		t.Error("the caller's http.Client was changed")
	}
}

func TestContextDeadlineStopsAtOnce(t *testing.T) {
	var calls atomic.Int32
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.InstantSnap.Get(ctx, "id")
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, wsw.ErrConnection) {
		t.Fatalf("got %v, want a connection error wrapping context.DeadlineExceeded", err)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("%d attempts, want 1", n)
	}
}

func TestRetryWaitBeyondDeadlineIsNotTaken(t *testing.T) {
	var calls atomic.Int32
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"success":false,"error_code":"rate_limited","error_message":"Slow down.","retry_after_seconds":30}`)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	_, err := client.AutoSchedule.TriggerNow(ctx, "page")
	var apiErr *wsw.Error
	if !errors.As(err, &apiErr) || !errors.Is(err, wsw.ErrRateLimit) || apiErr.RetryAfter != 30*time.Second {
		t.Fatalf("got %v, want a rate limit error asking for 30s", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("returned after %v", elapsed)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("%d attempts, want 1", n)
	}
}

func TestRetryAfterHeaderIsUsedWithoutBodyHint(t *testing.T) {
	var calls atomic.Int32
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		io.WriteString(w, `{"success":true,"status":"triggered"}`)
	})
	start := time.Now()
	if _, err := client.AutoSchedule.TriggerNow(context.Background(), "page"); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
		t.Errorf("waited %v despite Retry-After: 0", elapsed)
	}
}

func TestErrorString(t *testing.T) {
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"success":false,"error_code":"capture_not_found","error_message":"\"id\" was not found.","retry_after_seconds":null}`)
	})
	_, err := client.InstantSnap.Get(context.Background(), "id")
	if want := `wsw: capture_not_found (HTTP 404): "id" was not found.`; err == nil || err.Error() != want {
		t.Errorf("got %v, want %s", err, want)
	}
}

func TestPathSegmentsAreEscaped(t *testing.T) {
	var path string
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.EscapedPath()
		io.WriteString(w, `{"success":true}`)
	})
	if _, err := client.AutoSchedule.TriggerNow(context.Background(), "a/b"); err != nil {
		t.Fatal(err)
	}
	if want := "/auto-schedule/v1/web-pages/a%2Fb/trigger-now"; path != want {
		t.Errorf("path = %s, want %s", path, want)
	}
}

func TestSubmitUsesTheOrdinaryTimeout(t *testing.T) {
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}, wsw.WithTimeout(100*time.Millisecond), wsw.WithCaptureTimeout(10*time.Second), wsw.WithMaxRetries(0))

	start := time.Now()
	_, err := client.InstantSnap.Submit(context.Background(), &wsw.CaptureOptions{URL: "https://example.com"})
	if !errors.Is(err, wsw.ErrConnection) {
		t.Fatalf("got %v, want a connection error from the attempt timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("returned after %v, want the 100ms ordinary timeout", elapsed)
	}
}

func TestQueueFullIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"success":false,"error_code":"queue_full","error_message":"Too many.","retry_after_seconds":0}`)
	})
	_, err := client.InstantSnap.Submit(context.Background(), &wsw.CaptureOptions{URL: "https://example.com"})
	var apiErr *wsw.Error
	if !errors.As(err, &apiErr) || !errors.Is(err, wsw.ErrRateLimit) || apiErr.Code != "queue_full" || apiErr.Retryable() {
		t.Fatalf("got %v, want a non-retryable queue_full rate limit error", err)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("%d attempts, want 1", n)
	}
}

func TestWaitReturnsTheLastCaptureWhenTimeRunsOut(t *testing.T) {
	var calls atomic.Int32
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		io.WriteString(w, `{"success":true,"id":"a","status":"running"}`)
	})
	c, err := client.InstantSnap.Wait(context.Background(), "a",
		wsw.WithWaitTimeout(100*time.Millisecond), wsw.WithPollInterval(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != wsw.CaptureStatusRunning {
		t.Errorf("Status = %q, want running", c.Status)
	}
	if n := calls.Load(); n < 2 {
		t.Errorf("%d reads, want several", n)
	}
}

func TestWaitStopsAtContextDeadline(t *testing.T) {
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"success":true,"id":"a","status":"queued"}`)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := client.InstantSnap.Wait(ctx, "a", wsw.WithPollInterval(10*time.Second))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("returned after %v", elapsed)
	}
}

func TestWaitReturnsGetErrors(t *testing.T) {
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"success":false,"error_code":"capture_not_found","error_message":"Not found."}`)
	})
	_, err := client.InstantSnap.Wait(context.Background(), "a", wsw.WithPollInterval(0))
	if !errors.Is(err, wsw.ErrNotFound) {
		t.Fatalf("got %v, want a not found error", err)
	}
}
