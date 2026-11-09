package wsw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// InstantSnapService takes captures on demand. Use it through [Client.InstantSnap].
type InstantSnapService struct {
	client *Client
}

// RequestOption configures a single call.
type RequestOption func(*requestConfig)

type requestConfig struct {
	idempotencyKey string
}

// WithIdempotencyKey sends key as the capture's Idempotency-Key instead of a new random one.
// A second capture with the same key within 24 hours returns the first capture rather than
// capturing again. The key is up to 255 printable ASCII characters.
func WithIdempotencyKey(key string) RequestOption {
	return func(c *requestConfig) { c.idempotencyKey = key }
}

// Capture captures a page and answers with the capture, whose DownloadURL is valid for an hour.
// It can take up to 120 seconds.
func (s *InstantSnapService) Capture(ctx context.Context, opts *CaptureOptions, reqOpts ...RequestOption) (*Capture, error) {
	return s.capture(ctx, opts, ResponseTypeJSON, reqOpts)
}

// CaptureToBucket captures a page and uploads the file to the bucket in opts.S3Delivery. The
// answer's S3Delivery reports the upload.
func (s *InstantSnapService) CaptureToBucket(ctx context.Context, opts *CaptureOptions, reqOpts ...RequestOption) (*Capture, error) {
	return s.capture(ctx, opts, ResponseTypeCloudStorage, reqOpts)
}

// CaptureFile captures a page and answers with the file itself, up to 64 MB. The caller must
// read and close the returned file's Body, or call [CaptureFile.SaveTo]. The file is not kept
// by the API.
func (s *InstantSnapService) CaptureFile(ctx context.Context, opts *CaptureOptions, reqOpts ...RequestOption) (*CaptureFile, error) {
	r, err := s.captureRequest(opts, ResponseTypeByFormat, false, reqOpts)
	if err != nil {
		return nil, err
	}
	r.accept = "*/*"
	r.stream = true
	resp, err := s.client.send(ctx, r)
	if err != nil {
		return nil, err
	}
	return newCaptureFile(resp.Response), nil
}

// Submit accepts a capture to run when a browser is free and answers at once with the capture,
// whose Status is [CaptureStatusQueued]. Read it back with [InstantSnapService.Get], wait for
// it with [InstantSnapService.Wait], or receive the webhook. opts.Timeout bounds the browser
// time the capture may use. A capture still queued 15 minutes after it was accepted ends
// failed with the error code capture_expired and is not charged.
//
// The request is bounded by the ordinary timeout (see [WithTimeout]), not the capture
// timeout, since no capture runs while the caller waits. A queue_full refusal means the
// organization already has as many async captures waiting or running as it may; it is a
// [KindRateLimit] error carrying RetryAfter and is not retried.
func (s *InstantSnapService) Submit(ctx context.Context, opts *CaptureOptions, reqOpts ...RequestOption) (*Capture, error) {
	return s.submit(ctx, opts, ResponseTypeJSON, reqOpts)
}

// SubmitToBucket is [InstantSnapService.Submit] for a capture uploaded to the bucket in
// opts.S3Delivery, as [InstantSnapService.CaptureToBucket] does.
func (s *InstantSnapService) SubmitToBucket(ctx context.Context, opts *CaptureOptions, reqOpts ...RequestOption) (*Capture, error) {
	return s.submit(ctx, opts, ResponseTypeCloudStorage, reqOpts)
}

func (s *InstantSnapService) capture(ctx context.Context, opts *CaptureOptions, rt ResponseType, reqOpts []RequestOption) (*Capture, error) {
	r, err := s.captureRequest(opts, rt, false, reqOpts)
	if err != nil {
		return nil, err
	}
	return s.sendCapture(ctx, r)
}

func (s *InstantSnapService) submit(ctx context.Context, opts *CaptureOptions, rt ResponseType, reqOpts []RequestOption) (*Capture, error) {
	r, err := s.captureRequest(opts, rt, true, reqOpts)
	if err != nil {
		return nil, err
	}
	return s.sendCapture(ctx, r)
}

func (s *InstantSnapService) sendCapture(ctx context.Context, r *request) (*Capture, error) {
	var c Capture
	resp, err := s.client.sendJSON(ctx, r, &c)
	if err != nil {
		return nil, err
	}
	c.Replayed = strings.EqualFold(resp.Header.Get("Idempotent-Replayed"), "true")
	return &c, nil
}

// captureRequest builds a capture request. An async capture is answered at once, so it is
// bounded by the ordinary timeout rather than the capture timeout.
func (s *InstantSnapService) captureRequest(opts *CaptureOptions, rt ResponseType, async bool, reqOpts []RequestOption) (*request, error) {
	key, err := s.key()
	if err != nil {
		return nil, err
	}
	if opts == nil {
		return nil, errors.New("wsw: capture options are nil")
	}
	body, err := json.Marshal(struct {
		*CaptureOptions
		ResponseType ResponseType `json:"response_type"`
		Async        bool         `json:"async,omitempty"`
	}{opts, rt, async})
	if err != nil {
		return nil, fmt.Errorf("wsw: encoding capture options: %w", err)
	}

	var cfg requestConfig
	for _, o := range reqOpts {
		o(&cfg)
	}
	if cfg.idempotencyKey == "" {
		if cfg.idempotencyKey, err = newIdempotencyKey(); err != nil {
			return nil, err
		}
	}

	timeout := s.client.captureTimeout
	if async {
		timeout = s.client.timeout
	}
	return &request{
		method:         http.MethodPost,
		url:            s.client.instantSnapBaseURL + "/instant-snap/v1/captures",
		apiKey:         key,
		body:           body,
		idempotencyKey: cfg.idempotencyKey,
		timeout:        timeout,
	}, nil
}

// Get reads a capture back, with a fresh DownloadURL.
func (s *InstantSnapService) Get(ctx context.Context, id string) (*Capture, error) {
	key, err := s.key()
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, errors.New("wsw: capture id is empty")
	}
	var c Capture
	_, err = s.client.sendJSON(ctx, &request{
		method:  http.MethodGet,
		url:     s.client.instantSnapBaseURL + "/instant-snap/v1/captures/" + url.PathEscape(id),
		apiKey:  key,
		timeout: s.client.timeout,
	}, &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Defaults used by [InstantSnapService.Wait].
const (
	// DefaultWaitTimeout is how long Wait reads a capture before returning it unfinished.
	DefaultWaitTimeout = 360 * time.Second
	// DefaultPollInterval is how long Wait pauses between reads.
	DefaultPollInterval = 3 * time.Second
)

// WaitOption configures [InstantSnapService.Wait].
type WaitOption func(*waitConfig)

type waitConfig struct {
	timeout      time.Duration
	pollInterval time.Duration
}

// WithWaitTimeout sets how long Wait keeps reading before it returns the capture unfinished.
// Zero or less reads it once. The default is [DefaultWaitTimeout].
func WithWaitTimeout(d time.Duration) WaitOption {
	return func(c *waitConfig) { c.timeout = max(d, 0) }
}

// WithPollInterval sets the pause between reads. Zero or less does not pause. The default is
// [DefaultPollInterval].
func WithPollInterval(d time.Duration) WaitOption {
	return func(c *waitConfig) { c.pollInterval = max(d, 0) }
}

// Wait reads a capture with [InstantSnapService.Get] until its status is no longer queued,
// running or pending, and returns it. It is meant for a capture from
// [InstantSnapService.Submit] or [InstantSnapService.SubmitToBucket].
//
// When the wait timeout (see [WithWaitTimeout]) runs out first, Wait returns the capture as
// last read, still unfinished, and no error, so the caller can decide whether to wait again.
// An error from Get is returned as it is. A context deadline or cancellation stops the wait
// at once with an error that matches ctx.Err().
func (s *InstantSnapService) Wait(ctx context.Context, id string, opts ...WaitOption) (*Capture, error) {
	cfg := waitConfig{timeout: DefaultWaitTimeout, pollInterval: DefaultPollInterval}
	for _, o := range opts {
		o(&cfg)
	}
	deadline := time.Now().Add(cfg.timeout)
	for {
		c, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if !c.Status.inProgress() {
			return c, nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return c, nil
		}
		if pause := min(cfg.pollInterval, remaining); pause > 0 {
			timer := time.NewTimer(pause)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, fmt.Errorf("wsw: waiting for capture %s: %w", id, ctx.Err())
			case <-timer.C:
			}
		} else if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("wsw: waiting for capture %s: %w", id, err)
		}
	}
}

// Defaults returns the values a capture takes for the options it does not send, and the
// limits it is checked against. It needs no key.
func (s *InstantSnapService) Defaults(ctx context.Context) (*Defaults, error) {
	var d Defaults
	_, err := s.client.sendJSON(ctx, &request{
		method:  http.MethodGet,
		url:     s.client.instantSnapBaseURL + "/instant-snap/v1/defaults",
		timeout: s.client.timeout,
	}, &d)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// Catalog lists the devices [CaptureOptions.EmulatedDevice] accepts and the countries
// [Proxy.Country] accepts.
func (s *InstantSnapService) Catalog(ctx context.Context) (*Catalog, error) {
	key, err := s.key()
	if err != nil {
		return nil, err
	}
	var c Catalog
	_, err = s.client.sendJSON(ctx, &request{
		method:  http.MethodGet,
		url:     s.client.instantSnapBaseURL + "/instant-snap/v1/catalog",
		apiKey:  key,
		timeout: s.client.timeout,
	}, &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Download writes the file of a capture to w and returns the number of bytes written. The
// capture needs a DownloadURL, which only [InstantSnapService.Capture],
// [InstantSnapService.Get] and [InstantSnapService.Wait] return; it is valid for an hour.
func (s *InstantSnapService) Download(ctx context.Context, capture *Capture, w io.Writer) (int64, error) {
	if capture == nil || capture.DownloadURL == "" {
		return 0, errors.New("wsw: the capture has no download URL")
	}
	resp, err := s.client.send(ctx, &request{
		method:  http.MethodGet,
		url:     capture.DownloadURL,
		accept:  "*/*",
		timeout: s.client.captureTimeout,
		stream:  true,
	})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	n, err := io.Copy(w, resp.Body)
	if err != nil {
		return n, fmt.Errorf("wsw: downloading capture %s: %w", capture.ID, err)
	}
	return n, nil
}

// DownloadToFile writes the file of a capture to path, replacing any file there. Nothing is
// left at path if the download fails.
func (s *InstantSnapService) DownloadToFile(ctx context.Context, capture *Capture, path string) error {
	return writeFile(path, func(w io.Writer) error {
		_, err := s.Download(ctx, capture, w)
		return err
	})
}

func (s *InstantSnapService) key() (string, error) {
	if s.client.instantSnapKey == "" {
		return "", fmt.Errorf("%w: the Instant Snap key is not set; use wsw.WithInstantSnapKey or set %s", ErrMissingKey, EnvInstantSnapKey)
	}
	return s.client.instantSnapKey, nil
}
