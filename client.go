package wsw

import (
	"net/http"
	"os"
	"strings"
	"time"
)

// Defaults used by [NewClient].
const (
	DefaultInstantSnapBaseURL  = "https://api.websitescreenshotworld.com"
	DefaultAutoScheduleBaseURL = "https://autoapi.websitescreenshotworld.com"

	// DefaultCaptureTimeout bounds each capture attempt. The API can take up to 120 seconds.
	DefaultCaptureTimeout = 180 * time.Second
	// DefaultTimeout bounds each attempt of every other call.
	DefaultTimeout = 30 * time.Second
	// DefaultMaxRetries is how many times a retryable failure is retried.
	DefaultMaxRetries = 2
)

// Environment variables [NewClient] reads the keys from.
const (
	EnvInstantSnapKey  = "WSW_INSTANT_SNAP_API_KEY"
	EnvAutoScheduleKey = "WSW_AUTO_SCHEDULE_API_KEY"
)

// Client calls the Website Screenshot World APIs. It is safe for concurrent use.
type Client struct {
	// InstantSnap takes captures on demand.
	InstantSnap *InstantSnapService
	// AutoSchedule starts the capture of a scheduled web page.
	AutoSchedule *AutoScheduleService

	instantSnapKey      string
	autoScheduleKey     string
	instantSnapBaseURL  string
	autoScheduleBaseURL string
	httpClient          *http.Client
	captureTimeout      time.Duration
	timeout             time.Duration
	maxRetries          int
	userAgent           string
}

// ClientOption configures a [Client].
type ClientOption func(*Client)

// WithInstantSnapKey sets the Instant Snap API key. By default it is read from the
// WSW_INSTANT_SNAP_API_KEY environment variable.
func WithInstantSnapKey(key string) ClientOption {
	return func(c *Client) { c.instantSnapKey = key }
}

// WithAutoScheduleKey sets the Auto Schedule API key. By default it is read from the
// WSW_AUTO_SCHEDULE_API_KEY environment variable.
func WithAutoScheduleKey(key string) ClientOption {
	return func(c *Client) { c.autoScheduleKey = key }
}

// WithHTTPClient sets the HTTP client used for every request, for a custom transport or proxy.
// Leave its Timeout at zero: the library bounds each call itself (see [WithTimeout]). The client
// is copied, and the copy never follows redirects, so the API key is never sent anywhere else.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithBaseURLs points the client at other servers, such as a test double. An empty string
// keeps that product's default.
func WithBaseURLs(instantSnap, autoSchedule string) ClientOption {
	return func(c *Client) {
		if instantSnap != "" {
			c.instantSnapBaseURL = strings.TrimRight(instantSnap, "/")
		}
		if autoSchedule != "" {
			c.autoScheduleBaseURL = strings.TrimRight(autoSchedule, "/")
		}
	}
}

// WithCaptureTimeout bounds a whole capture or download call: every attempt, the waits between
// retries and reading the file. Zero means no bound beyond the context's. The default is
// [DefaultCaptureTimeout].
func WithCaptureTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.captureTimeout = max(d, 0) }
}

// WithTimeout bounds every other call as a whole, retries included; it also bounds
// [InstantSnapService.Submit] and [InstantSnapService.SubmitToBucket], which are answered at
// once. Zero means no bound beyond the context's. The default is [DefaultTimeout].
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.timeout = max(d, 0) }
}

// WithMaxRetries sets how many times a retryable failure is retried; zero turns retries off.
// The default is [DefaultMaxRetries].
func WithMaxRetries(n int) ClientOption {
	return func(c *Client) { c.maxRetries = max(n, 0) }
}

// NewClient returns a client. Either key may be left out; a call to a product whose key is
// missing fails before anything is sent.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		instantSnapBaseURL:  DefaultInstantSnapBaseURL,
		autoScheduleBaseURL: DefaultAutoScheduleBaseURL,
		httpClient:          &http.Client{},
		captureTimeout:      DefaultCaptureTimeout,
		timeout:             DefaultTimeout,
		maxRetries:          DefaultMaxRetries,
		userAgent:           userAgent(),
	}
	for _, opt := range opts {
		opt(c)
	}
	hc := *c.httpClient
	hc.CheckRedirect = noRedirects
	c.httpClient = &hc
	if c.instantSnapKey == "" {
		c.instantSnapKey = os.Getenv(EnvInstantSnapKey)
	}
	if c.autoScheduleKey == "" {
		c.autoScheduleKey = os.Getenv(EnvAutoScheduleKey)
	}
	c.InstantSnap = &InstantSnapService{client: c}
	c.AutoSchedule = &AutoScheduleService{client: c}
	return c
}
