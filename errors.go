package wsw

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ErrorKind groups the error codes the API can answer with. Branch on the kind, or on
// [Error.Code] for the exact reason; never on the HTTP status or the message.
type ErrorKind string

// Error kinds.
const (
	// KindAuthentication means the API key is malformed, revoked or for the other product.
	KindAuthentication ErrorKind = "authentication"
	// KindInvalidRequest means the request was refused as it stands; retrying will not help.
	KindInvalidRequest ErrorKind = "invalid_request"
	// KindPlanLimit means the plan has no captures left or does not include the feature.
	KindPlanLimit ErrorKind = "plan_limit"
	// KindNotFound means there is no such capture or web page for this account.
	KindNotFound ErrorKind = "not_found"
	// KindRateLimit means too many requests were sent. rate_limited is retried automatically;
	// queue_full (too many async captures waiting or running) is not.
	KindRateLimit ErrorKind = "rate_limit"
	// KindCapture means the website could not be captured, or not in time. It is not retried.
	KindCapture ErrorKind = "capture"
	// KindService means the service was busy or failed. Some of its codes are retried
	// automatically; see [Error.Retryable].
	KindService ErrorKind = "service"
	// KindConnection means the request could not be sent, or no answer came back. It is
	// retried automatically.
	KindConnection ErrorKind = "connection"
	// KindSignatureVerification means a webhook's signature did not verify.
	KindSignatureVerification ErrorKind = "signature_verification"
	// KindUnknown is a refusal whose error code this version of the library does not know.
	KindUnknown ErrorKind = "unknown"
)

// Sentinels for use with [errors.Is]. An [*Error] matches the sentinel of its Kind.
// KindUnknown has none.
var (
	ErrAuthentication        = errors.New("wsw: authentication failed")
	ErrInvalidRequest        = errors.New("wsw: invalid request")
	ErrPlanLimit             = errors.New("wsw: plan limit reached")
	ErrNotFound              = errors.New("wsw: not found")
	ErrRateLimit             = errors.New("wsw: rate limited")
	ErrCapture               = errors.New("wsw: capture failed")
	ErrService               = errors.New("wsw: service error")
	ErrConnection            = errors.New("wsw: connection failed")
	ErrSignatureVerification = errors.New("wsw: webhook signature verification failed")
)

// ErrMissingKey is wrapped by the error a call returns, before sending anything, when the
// client has no API key for that product.
var ErrMissingKey = errors.New("wsw: missing API key")

var sentinels = map[ErrorKind]error{
	KindAuthentication:        ErrAuthentication,
	KindInvalidRequest:        ErrInvalidRequest,
	KindPlanLimit:             ErrPlanLimit,
	KindNotFound:              ErrNotFound,
	KindRateLimit:             ErrRateLimit,
	KindCapture:               ErrCapture,
	KindService:               ErrService,
	KindConnection:            ErrConnection,
	KindSignatureVerification: ErrSignatureVerification,
}

// Error is a refusal from the API, a connection failure, or a webhook that did not verify.
//
//	var apiErr *wsw.Error
//	if errors.As(err, &apiErr) && apiErr.Code == "quota_exhausted" {
//		// ...
//	}
type Error struct {
	Kind ErrorKind
	// Code is the API's error_code, a stable lowercase token such as "rate_limited". When a
	// refusal had no JSON body it is derived from the HTTP status. Empty for connection and
	// signature errors.
	Code string
	// Message is the API's error_message, one sentence for a person. Its wording can change.
	Message string
	// HTTPStatus is zero when no answer came back.
	HTTPStatus int
	// RetryAfter is how long the API asked to wait, from retry_after_seconds or the
	// Retry-After header. Zero when it gave no hint.
	RetryAfter time.Duration
	// Body is the raw body of the refusal.
	Body []byte

	hasRetryAfter bool
	err           error
}

func (e *Error) Error() string {
	switch {
	case e.err != nil:
		return fmt.Sprintf("wsw: %s: %v", e.Message, e.err)
	case e.Code == "":
		return "wsw: " + e.Message
	default:
		return fmt.Sprintf("wsw: %s (HTTP %d): %s", e.Code, e.HTTPStatus, e.Message)
	}
}

// Unwrap returns the underlying network error of a connection failure, so that
// errors.Is(err, context.DeadlineExceeded) works.
func (e *Error) Unwrap() error { return e.err }

// Is reports whether target is the sentinel of e's Kind.
func (e *Error) Is(target error) bool {
	s, ok := sentinels[e.Kind]
	return ok && s == target
}

// Retryable reports whether the library retries this error by itself.
func (e *Error) Retryable() bool {
	return e.Kind == KindConnection || retryableCodes[e.Code]
}

var kindByCode = map[string]ErrorKind{
	"api_key_invalid": KindAuthentication,

	"validation_failed":                KindInvalidRequest,
	"s3_destination_invalid":           KindInvalidRequest,
	"web_page_id_invalid":              KindInvalidRequest,
	"idempotency_key_reused":           KindInvalidRequest,
	"sitemap_schedule_not_triggerable": KindInvalidRequest,

	"quota_exhausted":       KindPlanLimit,
	"feature_not_available": KindPlanLimit,
	"no_trigger_allowance":  KindPlanLimit,
	"forbidden":             KindPlanLimit,

	"capture_not_found":  KindNotFound,
	"web_page_not_found": KindNotFound,
	"not_found":          KindNotFound,

	"rate_limited": KindRateLimit,
	"queue_full":   KindRateLimit,

	"capture_failed":    KindCapture,
	"capture_timeout":   KindCapture,
	"payload_too_large": KindCapture,
	"capture_expired":   KindCapture,

	"capture_busy":           KindService,
	"service_unavailable":    KindService,
	"internal_error":         KindService,
	"request_cancelled":      KindService,
	"idempotency_key_in_use": KindService,
}

var retryableCodes = map[string]bool{
	"rate_limited":           true,
	"capture_busy":           true,
	"service_unavailable":    true,
	"idempotency_key_in_use": true,
}

// codeByStatus names the code of a refusal that has no JSON body, such as a proxy's error page.
var codeByStatus = map[int]string{
	400: "validation_failed",
	401: "api_key_invalid",
	403: "forbidden",
	404: "not_found",
	409: "idempotency_key_in_use",
	413: "payload_too_large",
	429: "rate_limited",
	500: "internal_error",
	502: "service_unavailable",
	503: "service_unavailable",
	504: "service_unavailable",
}

func kindOf(code string) ErrorKind {
	if k, ok := kindByCode[code]; ok {
		return k
	}
	return KindUnknown
}

func newConnectionError(err error) *Error {
	return &Error{Kind: KindConnection, Message: "connection failed", err: err}
}

func newSignatureError(message string) *Error {
	return &Error{Kind: KindSignatureVerification, Message: "webhook signature: " + message}
}

// newAPIError reads a refusal. The Retry-After header counts only when the body gives no hint.
func newAPIError(status int, header http.Header, body []byte) *Error {
	e := &Error{HTTPStatus: status, Body: body}

	var payload struct {
		ErrorCode         string   `json:"error_code"`
		ErrorMessage      string   `json:"error_message"`
		RetryAfterSeconds *float64 `json:"retry_after_seconds"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.ErrorCode != "" {
		e.Code = payload.ErrorCode
		e.Message = payload.ErrorMessage
		if payload.RetryAfterSeconds != nil {
			e.RetryAfter = seconds(*payload.RetryAfterSeconds)
			e.hasRetryAfter = true
		}
	} else {
		e.Code = codeByStatus[status]
		if e.Code == "" {
			e.Code = "unknown_error"
		}
		e.Message = fmt.Sprintf("the server answered %d %s without an error body", status, http.StatusText(status))
	}
	if !e.hasRetryAfter {
		e.RetryAfter, e.hasRetryAfter = parseRetryAfter(header.Get("Retry-After"))
	}
	e.Kind = kindOf(e.Code)
	return e
}
