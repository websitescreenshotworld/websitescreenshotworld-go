# Website Screenshot World for Go

The Go client for [Website Screenshot World](https://websitescreenshotworld.com): capture web pages on
demand with **Instant Snap**, start a scheduled page's capture with **Auto Schedule**, and verify the webhooks
both send.

It uses the standard library only.

## Install

```sh
go get github.com/websitescreenshotworld/websitescreenshotworld-go
```

The package is named `wsw`:

```go
import "github.com/websitescreenshotworld/websitescreenshotworld-go"
```

Go 1.22 or later is needed.

## Quick start: Instant Snap

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/websitescreenshotworld/websitescreenshotworld-go"
)

func main() {
	// Reads WSW_INSTANT_SNAP_API_KEY and WSW_AUTO_SCHEDULE_API_KEY.
	client := wsw.NewClient()
	ctx := context.Background()

	capture, err := client.InstantSnap.Capture(ctx, &wsw.CaptureOptions{
		URL:        "https://example.com",
		Format:     wsw.FormatWebP,
		ScrollMode: wsw.ScrollModeScroll,
		Viewport:   &wsw.Viewport{Width: wsw.Int(1440), Height: wsw.Int(900)},
		BlockAds:   wsw.Bool(false),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(capture.ID, capture.DownloadURL) // the link is valid for an hour

	if err := client.InstantSnap.DownloadToFile(ctx, capture, "example.webp"); err != nil {
		log.Fatal(err)
	}
}
```

Only `URL` is required. An option you leave unset is not sent, and the API uses its default
(`client.InstantSnap.Defaults(ctx)` lists them):

- scalar options are pointers, set with `wsw.String`, `wsw.Bool`, `wsw.Int` and `wsw.Float64`; `false`, `0` and
  `""` are values and are sent;
- token options (`Format`, `MediaCategory`, `ScrollMode`, `DeviceCategory` and the like) are typed strings with
  constants such as `wsw.FormatWebP`, and the empty string means "not set";
- nested blocks (`Viewport`, `Geolocation`, `Selectors`, `Proxy`, `TimestampWatermark`, `PageSource`,
  `S3Delivery`) are struct pointers.

`Headers` and `Cookies` names are sent exactly as you give them. `client.InstantSnap.Catalog(ctx)` lists the
devices `EmulatedDevice` accepts and the countries `Proxy.Country` accepts.

## Getting the file in the response

`CaptureFile` answers with the file itself (up to 64 MB) instead of a link. The body is a stream: read it once
and close it, or save it.

```go
file, err := client.InstantSnap.CaptureFile(ctx, &wsw.CaptureOptions{
	URL:           "https://example.com",
	MediaCategory: wsw.MediaCategoryPDF,
	Format:        wsw.FormatDocumentPDF,
})
if err != nil {
	log.Fatal(err)
}
fmt.Println(file.ID, file.ContentType, file.SizeBytes, file.SHA256)

if err := file.SaveTo(file.Filename); err != nil { // SaveTo closes the body
	log.Fatal(err)
}
```

To stream it elsewhere, read `file.Body` and close it; the capture timeout keeps running until you do:

```go
defer file.Close()
_, err = io.Copy(w, file.Body)
```

The API does not keep a file it returned this way.

## Uploading to your bucket

`CaptureToBucket` uploads the file to an S3-compatible bucket. Give `Region`, or `EndpointURL` for providers
other than AWS.

```go
capture, err := client.InstantSnap.CaptureToBucket(ctx, &wsw.CaptureOptions{
	URL: "https://example.com",
	S3Delivery: &wsw.S3DeliveryOptions{
		AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		Bucket:          "captures",
		Region:          wsw.String("eu-west-2"),
		KeyPrefix:       wsw.String("homepage/"),
		WaitForSync:     wsw.Bool(true),
	},
})
if err != nil {
	log.Fatal(err)
}
fmt.Println(capture.S3Delivery.Status, capture.S3Delivery.Key)
```

## Reading a capture back

`Get` returns a capture with a fresh `DownloadURL`. `Download` writes the file to any `io.Writer`, and
`DownloadToFile` to a path (replacing the file only once the download has succeeded).

```go
capture, err := client.InstantSnap.Get(ctx, "3f6c2a1e-9b8d-4c7e-a5f4-1d2e3c4b5a69")
if err != nil {
	log.Fatal(err)
}
if _, err := client.InstantSnap.Download(ctx, capture, os.Stdout); err != nil {
	log.Fatal(err)
}
```

Files are deleted 24 hours after the capture (`capture.ExpiresAt`).

## Async captures

`Submit` and `SubmitToBucket` hand a capture to the API and return at once with the capture in status
`wsw.CaptureStatusQueued`; it moves to `wsw.CaptureStatusRunning` when a browser picks it up, then ends
`succeeded` or `failed`. `Wait` reads it with `Get` (every 3 seconds by default) until it has finished:

```go
queued, err := client.InstantSnap.Submit(ctx, &wsw.CaptureOptions{
	URL:     "https://example.com",
	Timeout: wsw.Int(240), // browser time in seconds, 30 to 300; the server uses 120 if unset
})
if err != nil {
	log.Fatal(err)
}

capture, err := client.InstantSnap.Wait(ctx, queued.ID,
	wsw.WithWaitTimeout(10*time.Minute), // default 360 seconds
	wsw.WithPollInterval(5*time.Second), // default 3 seconds
)
if err != nil {
	log.Fatal(err)
}
if capture.Status == wsw.CaptureStatusSucceeded {
	fmt.Println(capture.DownloadURL)
}
```

When the wait timeout runs out first, `Wait` returns the capture as last read (still `queued` or `running`) and
no error, so you can call it again. An error from `Get` is returned as it is, and a context deadline or
cancellation stops the wait at once.

`SubmitToBucket` does the same for an upload to your bucket:

```go
queued, err := client.InstantSnap.SubmitToBucket(ctx, &wsw.CaptureOptions{
	URL: "https://example.com",
	S3Delivery: &wsw.S3DeliveryOptions{
		AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		Bucket:          "captures",
		Region:          wsw.String("eu-west-2"),
	},
})
```

- `Timeout` is only for `Submit` and `SubmitToBucket`; the other capture methods are refused if it is set.
- A capture still waiting in the queue 15 minutes after it was accepted ends `failed` with the error code
  `capture_expired`, and is not charged.
- `queue_full` (HTTP 429) means your organization already has as many async captures waiting or running as it
  may. It is a `wsw.ErrRateLimit` error whose `RetryAfter` says when to try again, and the client does not
  retry it by itself.
- The submit request is answered at once, so it is bounded by the ordinary 30-second timeout, not the capture
  timeout. Retries and the `Idempotency-Key` work as for `Capture`.
- Instead of waiting, you can pass `CallbackURL` and receive the `capture.succeeded` or `capture.failed`
  webhook.

## Auto Schedule

```go
result, err := client.AutoSchedule.TriggerNow(ctx, "7d4e2b1a-6c5f-4a3e-9d8c-2b1a0f9e8d7c")
if err != nil {
	log.Fatal(err)
}
switch result.Status {
case wsw.TriggerStatusTriggered:
	fmt.Println("queued at", result.TriggeredAt)
case wsw.TriggerStatusAlreadyPending:
	fmt.Println("already queued; nothing new was started")
}
```

The web page id is shown in the dashboard; webhooks call it `schedule_id`. The results arrive by webhook.

## Errors

Every failure reported by the API or the network is a `*wsw.Error`. Its `Kind` groups the error codes, and a
sentinel for each kind works with `errors.Is`:

| Kind | Sentinel | Codes |
|---|---|---|
| `KindAuthentication` | `ErrAuthentication` | `api_key_invalid` |
| `KindInvalidRequest` | `ErrInvalidRequest` | `validation_failed`, `s3_destination_invalid`, `web_page_id_invalid`, `idempotency_key_reused`, `sitemap_schedule_not_triggerable` |
| `KindPlanLimit` | `ErrPlanLimit` | `quota_exhausted`, `feature_not_available`, `no_trigger_allowance`, `forbidden` |
| `KindNotFound` | `ErrNotFound` | `capture_not_found`, `web_page_not_found`, `not_found` |
| `KindRateLimit` | `ErrRateLimit` | `rate_limited`, `queue_full` |
| `KindCapture` | `ErrCapture` | `capture_failed`, `capture_timeout`, `payload_too_large`, `capture_expired` |
| `KindService` | `ErrService` | `capture_busy`, `service_unavailable`, `internal_error`, `request_cancelled`, `idempotency_key_in_use` |
| `KindConnection` | `ErrConnection` | none: the request could not be sent, or no answer came back |
| `KindSignatureVerification` | `ErrSignatureVerification` | none: a webhook did not verify |
| `KindUnknown` | none | a code this version does not know yet |

For the exact reason, read `Code` with `errors.As`. Branch on the kind or the code, never on `HTTPStatus` or
`Message` (its wording can change).

```go
_, err := client.InstantSnap.Capture(ctx, &wsw.CaptureOptions{URL: "https://example.com"})

var apiErr *wsw.Error
switch {
case err == nil:
case errors.Is(err, wsw.ErrPlanLimit):
	log.Print("upgrade the plan: ", err)
case errors.Is(err, wsw.ErrCapture):
	log.Print("the website could not be captured: ", err)
case errors.Is(err, context.DeadlineExceeded):
	log.Print("gave up waiting")
case errors.As(err, &apiErr):
	log.Printf("%s (HTTP %d): %s", apiErr.Code, apiErr.HTTPStatus, apiErr.Message)
default:
	log.Print(err)
}
```

`*wsw.Error` also carries `RetryAfter` (the server's hint, if any) and `Body` (the raw answer). A connection
error wraps its cause, so `errors.Is(err, context.DeadlineExceeded)` and `errors.Is(err, context.Canceled)`
work. A call to a product whose key is missing fails before sending anything, with an error that wraps
`wsw.ErrMissingKey`.

## Retries

The client retries `rate_limited`, `capture_busy`, `service_unavailable` and `idempotency_key_in_use`, and
connection failures, twice by default. It waits as long as the API asks (`retry_after_seconds` or
`Retry-After`, capped at 60 seconds), or otherwise 0.5 s and then 1 s, each with up to 25 % jitter. Everything
else, including `capture_failed`, `capture_timeout` and `queue_full`, is returned at once. `apiErr.Retryable()` tells you
which kind you have.

A context deadline or cancellation stops retrying at once, and a wait that would outlast the deadline is not
started. Change the number of retries with `wsw.WithMaxRetries(n)`; `0` turns them off.

## Idempotency keys

Every capture sends an `Idempotency-Key` header, a new random UUID per call that its retries reuse, so a retry
never captures twice. Pass your own key to make a capture safe to repeat across calls or processes:

```go
capture, err := client.InstantSnap.Capture(ctx, &wsw.CaptureOptions{URL: "https://example.com"},
	wsw.WithIdempotencyKey("order-1234-hero"))
if err == nil && capture.Replayed {
	fmt.Println("the API returned the earlier capture")
}
```

Within 24 hours, a second capture with the same key returns the first capture rather than capturing again;
`idempotency_key_in_use` (retried) means the first is still running, and `idempotency_key_reused` means the
options differ or the first was a `CaptureFile`, whose file is not kept.

## Timeouts

A capture, or a download, may take 180 seconds: the API can take up to 120 seconds to capture a slow page.
Every other call, including `Submit` and `SubmitToBucket`, may take 30 seconds. The time covers the whole call,
retries and the waits between them included; once it has passed, the call returns a connection error that
matches `context.DeadlineExceeded`, and is not retried. Change them with `wsw.WithCaptureTimeout` and
`wsw.WithTimeout`, or bound a call with the context as well:

```go
ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
defer cancel()
capture, err := client.InstantSnap.Capture(ctx, opts)
```

For `CaptureFile`, the capture timeout covers reading the body too, until you close it.

## Webhooks

Each delivery is a `POST` whose `X-WSW-Signature` header signs the body. `wsw.VerifyWebhook` checks the
signature and the timestamp (within 5 minutes by default; change it with `wsw.WithTolerance`) and returns the
typed event. Switch on its concrete type:

| Event | Type |
|---|---|
| Instant Snap `capture.succeeded` / `capture.failed` | `*InstantSnapCaptureSucceededEvent` / `*InstantSnapCaptureFailedEvent` |
| Auto Schedule `capture.succeeded` / `capture.failed` | `*AutoScheduleCaptureSucceededEvent` / `*AutoScheduleCaptureFailedEvent` |
| `capture.changed` | `*CaptureChangedEvent` |
| `delivery.completed` | `*DeliveryCompletedEvent` |
| `schedule.paused` | `*SchedulePausedEvent` |
| `sitemap_run.completed` | `*SitemapRunCompletedEvent` |
| `ping` (either product) | `*PingEvent` |
| a type added later | `*UnknownEvent`, with `Type` and the raw `Data` |

Every event embeds `EventEnvelope` (`ID`, `Type`, `CreatedAt`, `Product`). Deliveries can be retried, so use
`ID` to ignore one you have already handled.

Verify the body **exactly as received**. Decoding the JSON and encoding it again changes the bytes and breaks
the signature.

### net/http (and routers built on it, such as chi)

```go
secret := os.Getenv("WSW_WEBHOOK_SECRET")

http.HandleFunc("POST /webhooks/wsw", func(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return
	}
	event, err := wsw.VerifyWebhook(body, r.Header.Get(wsw.SignatureHeader), secret)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	switch e := event.(type) {
	case *wsw.InstantSnapCaptureSucceededEvent:
		log.Printf("capture %s is ready", e.Data.ID)
	case *wsw.CaptureChangedEvent:
		log.Printf("%s changed by %.1f%%", e.Data.ScheduleName, e.Data.ChangePercent)
	case *wsw.UnknownEvent:
		log.Printf("ignoring %s", e.Type)
	}
	w.WriteHeader(http.StatusNoContent)
})
```

### Gin

```go
router.POST("/webhooks/wsw", func(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	event, err := wsw.VerifyWebhook(body, c.GetHeader(wsw.SignatureHeader), secret)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	handle(event)
	c.Status(http.StatusNoContent)
})
```

Do not call `c.ShouldBindJSON` or `c.BindJSON` first: they consume the body.

### Echo

```go
e.POST("/webhooks/wsw", func(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest)
	}
	event, err := wsw.VerifyWebhook(body, c.Request().Header.Get(wsw.SignatureHeader), secret)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid signature")
	}
	handle(event)
	return c.NoContent(http.StatusNoContent)
})
```

Answer with a 2xx quickly and do slow work afterwards.

## Configuration

| Option | Default |
|---|---|
| `wsw.WithInstantSnapKey(key)` | `WSW_INSTANT_SNAP_API_KEY` environment variable |
| `wsw.WithAutoScheduleKey(key)` | `WSW_AUTO_SCHEDULE_API_KEY` environment variable |
| `wsw.WithBaseURLs(instantSnap, autoSchedule)` | `https://api.websitescreenshotworld.com`, `https://autoapi.websitescreenshotworld.com` (an empty string keeps the default) |
| `wsw.WithCaptureTimeout(d)` | 180 seconds per call, retries included |
| `wsw.WithTimeout(d)` | 30 seconds per call, retries included |
| `wsw.WithMaxRetries(n)` | 2 |
| `wsw.WithHTTPClient(hc)` | a new `http.Client` using `http.DefaultTransport` |

Either key may be left out if you use only one product. A `*wsw.Client` is safe for concurrent use: create one
and share it. If you pass your own `http.Client`, leave its `Timeout` at zero and let the options above bound
each call. The library uses a copy of it that never follows redirects, so the API key is never sent to another
host; your own `http.Client` is left as it is.

## Supported Go versions

Go 1.22 and later. Each release is tested on the oldest supported version and the latest one.

## Development

```sh
go test ./...
```

The tests start the shared conformance server in `spec/conformance`, so they need Node.js 22 or later.
`spec/` is the [websitescreenshotworld-spec](https://github.com/websitescreenshotworld/websitescreenshotworld-spec) submodule: clone with `git clone --recurse-submodules`, or run `git submodule update --init` in an existing clone. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT. See [LICENSE](LICENSE).
