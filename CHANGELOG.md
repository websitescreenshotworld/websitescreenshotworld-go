# Changelog

All notable changes to the Go client are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 0.1.0 - 2026-11-09

The first release.

### Added

- `wsw.NewClient` for both products, with keys read from `WSW_INSTANT_SNAP_API_KEY` and
  `WSW_AUTO_SCHEDULE_API_KEY` by default, and options for base URLs, timeouts, retries and the HTTP client.
- Instant Snap: `Capture`, `CaptureFile` (the file as a stream, with its metadata and `SaveTo`),
  `CaptureToBucket`, `Get`, `Download`, `DownloadToFile`, `Defaults` and `Catalog`, covering every capture
  option of the Instant Snap API.
- Instant Snap async captures: `Submit` and `SubmitToBucket` (answered at once with the capture `queued`) and
  `Wait` (reads it until it has finished, with `WithWaitTimeout` and `WithPollInterval`), the
  `CaptureOptions.Timeout` option (30-300 seconds, async only), the `CaptureStatusQueued` and
  `CaptureStatusRunning` statuses, and the `queue_full` (rate limit, not retried) and `capture_expired`
  (capture) error codes.
- Auto Schedule: `TriggerNow`.
- The `PressEscape` capture option, and the `sitemap_schedule_not_triggerable` error code (invalid request: a
  sitemap schedule runs only on its schedule), from API version 1.1.0.
- `wsw.VerifyWebhook`, which checks the `X-WSW-Signature` header and returns typed events for every webhook
  both products send, and `*wsw.UnknownEvent` for types added later.
- A single `*wsw.Error` type with a `Kind` per group of error codes, `errors.Is` sentinels, and connection
  errors that wrap their cause.
- Automatic retries of rate limits, busy or unavailable services and connection failures, with an
  `Idempotency-Key` on every capture so a retry never captures twice.
- Timeouts that bound the whole call, retries and the waits between them included (180 seconds for captures
  and downloads, 30 seconds for everything else); a call that runs out of time returns an error matching
  `context.DeadlineExceeded` and is not retried.
- Redirects are never followed, so the API key is never sent to another host; a 3xx answer is returned as an
  error. A client passed with `WithHTTPClient` is copied for this and left unchanged.
