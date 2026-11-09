package wsw

import (
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	maxRetryWait     = 60 * time.Second
	firstRetryWait   = 500 * time.Millisecond
	retryJitterRatio = 0.25
)

// retryWait is how long to wait before retry number n (0 for the first retry) after err.
func retryWait(err *Error, n int) time.Duration {
	if err.hasRetryAfter {
		return min(err.RetryAfter, maxRetryWait)
	}
	base := firstRetryWait << min(n, 7)
	jitter := time.Duration(rand.Float64() * retryJitterRatio * float64(base))
	return min(base+jitter, maxRetryWait)
}

// parseRetryAfter reads a Retry-After header, in seconds or as an HTTP date.
func parseRetryAfter(v string) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if s, err := strconv.ParseFloat(v, 64); err == nil {
		return seconds(s), true
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(time.Until(t), 0), true
	}
	return 0, false
}

func seconds(s float64) time.Duration {
	if s <= 0 || math.IsNaN(s) {
		return 0
	}
	if s >= math.MaxInt64/float64(time.Second) {
		return math.MaxInt64
	}
	return time.Duration(s * float64(time.Second))
}
