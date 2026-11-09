// Package wsw is the Go client for Website Screenshot World.
//
// It covers the two HTTP APIs, Instant Snap (on-demand captures) and Auto Schedule
// (start a scheduled web page's capture now), and verifies the signature of the webhooks
// both send.
//
// A [Client] is safe for concurrent use; create one and reuse it:
//
//	client := wsw.NewClient() // keys from WSW_INSTANT_SNAP_API_KEY and WSW_AUTO_SCHEDULE_API_KEY
//
//	capture, err := client.InstantSnap.Capture(ctx, &wsw.CaptureOptions{
//		URL:    "https://example.com",
//		Format: wsw.FormatWebP,
//	})
//
// # Options
//
// An option that is not set is not sent, so the API applies its own default. Scalar options
// are pointers, set with [String], [Bool], [Int] and [Float64]; false, 0 and "" are values
// and are sent. Token options such as [CaptureOptions.Format] use typed strings, where the
// empty string means "not set". Nested blocks are struct pointers.
//
// # Errors
//
// Every failure the API or the network reports is an [*Error]. Its Kind groups the codes;
// test it with [errors.Is] and the sentinels ([ErrRateLimit], [ErrNotFound] and the others),
// or read Code with [errors.As] for the exact reason. Network failures still match
// [context.DeadlineExceeded] and [context.Canceled].
//
// # Retries
//
// Rate limits (but not queue_full), busy capture slots, unavailable services and connection failures are retried
// twice by default, waiting for the server's hint when it gives one. Captures always carry an
// Idempotency-Key, so a retry never runs a capture twice. A context deadline or cancellation
// stops retrying at once.
package wsw
