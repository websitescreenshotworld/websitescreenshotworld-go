package wsw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// maxErrorBody caps how much of a refusal is kept; error bodies are small, proxy pages may not be.
const maxErrorBody = 1 << 20

// request is one API call, sent once or retried.
type request struct {
	method string
	url    string
	// apiKey is sent as X-Api-Key unless empty.
	apiKey         string
	body           []byte
	accept         string
	idempotencyKey string
	timeout        time.Duration
	// stream leaves a successful response's body open for the caller.
	stream bool
}

// response is a successful answer. body is set unless the request streamed.
type response struct {
	*http.Response
	body []byte
}

func userAgent() string {
	return fmt.Sprintf("websitescreenshotworld-go/%s (go %s)", Version, strings.TrimPrefix(runtime.Version(), "go"))
}

// send makes the request, retrying what the API allows, until it succeeds, fails for good,
// runs out of retries or its time is up. r.timeout bounds the whole call: every attempt, the
// waits between them and, for a stream, reading the body until it is closed.
func (c *Client) send(ctx context.Context, r *request) (*response, error) {
	var body io.Reader
	if r.body != nil {
		body = bytes.NewReader(r.body)
	}
	req, err := http.NewRequest(r.method, r.url, body)
	if err != nil {
		return nil, fmt.Errorf("wsw: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", r.accept)
	if r.apiKey != "" {
		req.Header.Set("X-Api-Key", r.apiKey)
	}
	if r.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if r.idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", r.idempotencyKey)
	}

	cancel := context.CancelFunc(func() {})
	if r.timeout > 0 {
		ctx, cancel = context.WithTimeoutCause(ctx, r.timeout, &callTimeoutError{timeout: r.timeout})
	}
	for n := 0; ; n++ {
		resp, err := c.attempt(ctx, req, r)
		if err == nil {
			if r.stream {
				// The call's time keeps running while the caller reads the body.
				resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
			} else {
				cancel()
			}
			return resp, nil
		}
		if ctx.Err() != nil || n >= c.maxRetries || !err.Retryable() {
			cancel()
			return nil, err
		}
		wait := retryWait(err, n)
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < wait {
			cancel()
			return nil, err
		}
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				cancel()
				return nil, err
			case <-timer.C:
			}
		}
	}
}

// callTimeoutError ends a call whose timeout has passed. It matches context.DeadlineExceeded,
// like a deadline set on the caller's context.
type callTimeoutError struct {
	timeout time.Duration
}

func (e *callTimeoutError) Error() string {
	return fmt.Sprintf("no answer within %s", e.timeout)
}

func (e *callTimeoutError) Unwrap() error { return context.DeadlineExceeded }

// attempt sends one copy of base within ctx.
func (c *Client) attempt(ctx context.Context, base *http.Request, r *request) (*response, *Error) {
	req := base.Clone(ctx)
	if base.GetBody != nil {
		req.Body, _ = base.GetBody()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, newConnectionError(withCause(ctx, err))
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if r.stream {
			return &response{Response: resp}, nil
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, newConnectionError(withCause(ctx, err))
		}
		return &response{Response: resp, body: data}, nil
	}

	// A redirect lands here too: the client never follows one (see noRedirects).
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if err != nil {
		return nil, newConnectionError(withCause(ctx, err))
	}
	return nil, newAPIError(resp.StatusCode, resp.Header, data)
}

// withCause names why ctx ended, when it did: the call's timeout or the caller's context.
func withCause(ctx context.Context, err error) error {
	cause := context.Cause(ctx)
	if cause == nil || errors.Is(err, cause) {
		return err
	}
	return fmt.Errorf("%w: %w", cause, err)
}

// noRedirects stops the HTTP client following a redirect, which would send the API key to
// wherever it pointed. Neither the API nor a download link redirects, so a 3xx is answered as the
// refusal it is.
func noRedirects(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

// cancelOnClose ends a streamed call once the caller is done with the body.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnClose) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

// sendJSON sends a request that answers in JSON and decodes a successful answer into out.
func (c *Client) sendJSON(ctx context.Context, r *request, out any) (*response, error) {
	r.accept = "application/json"
	resp, err := c.send(ctx, r)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(resp.body, out); err != nil {
		return nil, fmt.Errorf("wsw: decoding the answer to %s %s: %w", r.method, r.url, err)
	}
	return resp, nil
}
