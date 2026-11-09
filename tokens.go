package wsw

// The API returns tokens as strings. Each token type below is a string type with constants for
// the values known to this version, so a value added to the API later still comes through.

// MediaCategory is what kind of file a capture produces.
type MediaCategory string

// Media categories.
const (
	MediaCategoryImage MediaCategory = "image"
	MediaCategoryVideo MediaCategory = "video"
	MediaCategoryPDF   MediaCategory = "pdf"
)

// Format is the file format of a capture. It must belong to the capture's [MediaCategory].
type Format string

// Formats, grouped by media category.
const (
	FormatPNG  Format = "png"
	FormatJPEG Format = "jpeg"
	FormatWebP Format = "webp"
	FormatTIFF Format = "tiff"

	FormatMP4  Format = "mp4"
	FormatWebM Format = "webm"
	FormatMOV  Format = "mov"
	FormatGIF  Format = "gif"

	FormatImagePDF    Format = "image_pdf"
	FormatDocumentPDF Format = "document_pdf"
)

// DeviceCategory is the kind of browser a capture emulates.
type DeviceCategory string

// Device categories.
const (
	DeviceCategoryDesktop DeviceCategory = "desktop"
	DeviceCategoryMobile  DeviceCategory = "mobile"
)

// ScrollMode chooses between the visible viewport and the full page.
type ScrollMode string

// Scroll modes.
const (
	ScrollModeViewport ScrollMode = "viewport"
	ScrollModeScroll   ScrollMode = "scroll"
)

// WatermarkLocation is where the timestamp watermark strip goes.
type WatermarkLocation string

// Watermark locations.
const (
	WatermarkLocationTop    WatermarkLocation = "top"
	WatermarkLocationBottom WatermarkLocation = "bottom"
)

// PageSourceFormat is the form in which a capture keeps the page source.
type PageSourceFormat string

// Page source formats.
const (
	PageSourceFormatHTML PageSourceFormat = "html"
	PageSourceFormatJSON PageSourceFormat = "json"
	PageSourceFormatText PageSourceFormat = "text"
)

// ResponseType is how the API answered a capture. The capture methods choose it; it is not
// an option.
type ResponseType string

// Response types.
const (
	ResponseTypeJSON         ResponseType = "json"
	ResponseTypeByFormat     ResponseType = "by_format"
	ResponseTypeCloudStorage ResponseType = "cloud_storage"
)

// CaptureStatus is where an Instant Snap capture is in its life. A capture from
// [InstantSnapService.Submit] or [InstantSnapService.SubmitToBucket] goes from queued to
// running to succeeded or failed.
type CaptureStatus string

// Capture statuses. CaptureStatusQueued (accepted, waiting for a browser) and
// CaptureStatusRunning (a browser has it) are the states of an async capture before it ends.
// CaptureStatusDelivered is the end state of a file returned by
// [InstantSnapService.CaptureFile]: the file went back in the response and is not kept.
const (
	CaptureStatusQueued    CaptureStatus = "queued"
	CaptureStatusRunning   CaptureStatus = "running"
	CaptureStatusPending   CaptureStatus = "pending"
	CaptureStatusSucceeded CaptureStatus = "succeeded"
	CaptureStatusDelivered CaptureStatus = "delivered"
	CaptureStatusFailed    CaptureStatus = "failed"
	CaptureStatusTimedOut  CaptureStatus = "timed_out"
	CaptureStatusExpired   CaptureStatus = "expired"
)

// inProgress reports whether a capture with this status has not finished yet.
func (s CaptureStatus) inProgress() bool {
	return s == CaptureStatusQueued || s == CaptureStatusRunning || s == CaptureStatusPending
}

// S3DeliveryStatus is how far the upload to the caller's bucket has got.
type S3DeliveryStatus string

// S3 delivery statuses.
const (
	S3DeliveryStatusNotStarted S3DeliveryStatus = "not_started"
	S3DeliveryStatusInProgress S3DeliveryStatus = "in_progress"
	S3DeliveryStatusCompleted  S3DeliveryStatus = "completed"
	S3DeliveryStatusFailed     S3DeliveryStatus = "failed"
)

// TriggerStatus is the outcome of [AutoScheduleService.TriggerNow].
type TriggerStatus string

// Trigger statuses. TriggerStatusAlreadyPending means a capture for the web page was already
// queued or running: nothing new was started and nothing was charged.
const (
	TriggerStatusTriggered      TriggerStatus = "triggered"
	TriggerStatusAlreadyPending TriggerStatus = "already_pending"
)

// TriggeredBy says who started a pending Auto Schedule capture.
type TriggeredBy string

// Trigger sources.
const (
	TriggeredByAPI TriggeredBy = "api"
	TriggeredByUI  TriggeredBy = "ui"
)

// Product names the API that sent a webhook.
type Product string

// Products.
const (
	ProductInstantSnap  Product = "instant_snap"
	ProductAutoSchedule Product = "auto_schedule"
)
