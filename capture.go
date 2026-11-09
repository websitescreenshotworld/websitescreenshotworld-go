package wsw

import "time"

// Capture is an Instant Snap capture as the API describes it. Fields the API sends as null
// hold their zero value.
type Capture struct {
	ID            string        `json:"id"`
	Status        CaptureStatus `json:"status"`
	ResponseType  ResponseType  `json:"response_type"`
	MediaCategory MediaCategory `json:"media_category"`
	// Format is the format written.
	Format      Format `json:"format"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	// DurationMS is how long the capture took.
	DurationMS int `json:"duration_ms"`
	// PageHTTPStatus is the status the website answered with.
	PageHTTPStatus int       `json:"page_http_status"`
	CapturedAt     time.Time `json:"captured_at"`
	// FinalURL is where the browser ended up, when the page redirected.
	FinalURL string `json:"final_url"`
	// MediaSHA256 is the lowercase hex SHA-256 digest of the file.
	MediaSHA256 string `json:"media_sha256"`
	// DownloadURL is valid for 60 minutes and reissued by every [InstantSnapService.Get].
	// It is empty for captures uploaded to a bucket and in webhooks.
	DownloadURL string `json:"download_url"`
	// ExpiresAt is when the file is deleted, 24 hours after the capture.
	ExpiresAt  time.Time   `json:"expires_at"`
	PageSource *PageSource `json:"page_source"`
	// S3Delivery is set for captures uploaded to a bucket.
	S3Delivery  *S3Delivery `json:"s3_delivery"`
	CallbackURL string      `json:"callback_url"`
	// ErrorCode says why a failed or timed-out capture ended; it is one of the error codes.
	ErrorCode string `json:"error_code"`
	// SpecVersion names the default set the capture was taken under.
	SpecVersion string `json:"spec_version"`

	// Replayed is true when the API answered with the result of an earlier request that had
	// the same Idempotency-Key.
	Replayed bool `json:"-"`
}

// PageSource is the page source kept alongside a capture.
type PageSource struct {
	Format      PageSourceFormat `json:"format"`
	ContentType string           `json:"content_type"`
	SizeBytes   int64            `json:"size_bytes"`
	// SHA256 is the lowercase hex digest of the source.
	SHA256 string `json:"sha256"`
	// DownloadURL is short-lived and reissued on every read. It is empty in webhooks.
	DownloadURL string `json:"download_url"`
}

// S3Delivery is the state of the upload to the caller's bucket.
type S3Delivery struct {
	Status S3DeliveryStatus `json:"status"`
	Bucket string           `json:"bucket"`
	Key    string           `json:"key"`
	// AccessKeyID is masked to its last four characters. It is empty in webhooks.
	AccessKeyID string `json:"access_key_id"`
	Error       string `json:"error"`
}
