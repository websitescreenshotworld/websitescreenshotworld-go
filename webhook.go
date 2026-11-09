package wsw

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// SignatureHeader is the header that carries a webhook's signature.
const SignatureHeader = "X-WSW-Signature"

// DefaultWebhookTolerance is how far a webhook's timestamp may be from now.
const DefaultWebhookTolerance = 300 * time.Second

// VerifyOption configures [VerifyWebhook].
type VerifyOption func(*verifyConfig)

type verifyConfig struct {
	tolerance time.Duration
	now       func() time.Time
}

// WithTolerance sets how far the signature's timestamp may be before or after now. Values of
// zero or less keep the default, [DefaultWebhookTolerance].
func WithTolerance(d time.Duration) VerifyOption {
	return func(c *verifyConfig) {
		if d > 0 {
			c.tolerance = d
		}
	}
}

// WithNow sets the clock the timestamp is checked against, for tests.
func WithNow(now func() time.Time) VerifyOption {
	return func(c *verifyConfig) {
		if now != nil {
			c.now = now
		}
	}
}

// VerifyWebhook checks that body was signed with secret and returns the event it carries.
//
// body must be the request body exactly as received: parsing and re-encoding it changes the
// bytes and breaks the signature. header is the value of the X-WSW-Signature header. A
// signature that does not verify returns an [*Error] matching [ErrSignatureVerification].
func VerifyWebhook(body []byte, header, secret string, opts ...VerifyOption) (Event, error) {
	cfg := verifyConfig{tolerance: DefaultWebhookTolerance, now: time.Now}
	for _, o := range opts {
		o(&cfg)
	}
	if secret == "" {
		return nil, newSignatureError("the webhook secret is empty")
	}

	var (
		timestamp  string
		signatures []string
	)
	for _, part := range strings.Split(header, ",") {
		name, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(name) {
		case "t":
			timestamp = strings.TrimSpace(value)
		case "v1":
			signatures = append(signatures, strings.ToLower(strings.TrimSpace(value)))
		}
	}
	if timestamp == "" {
		return nil, newSignatureError("the header has no timestamp")
	}
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return nil, newSignatureError("the timestamp is not an integer")
	}
	if len(signatures) == 0 {
		return nil, newSignatureError("the header has no v1 signature")
	}

	age := cfg.now().Sub(time.Unix(unix, 0))
	if age > cfg.tolerance || age < -cfg.tolerance {
		return nil, newSignatureError("the timestamp is outside the tolerance")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	expected := []byte(hex.EncodeToString(mac.Sum(nil)))

	matched := false
	for _, sig := range signatures {
		if hmac.Equal(expected, []byte(sig)) {
			matched = true
		}
	}
	if !matched {
		return nil, newSignatureError("no v1 signature matches")
	}
	return parseEvent(body)
}
