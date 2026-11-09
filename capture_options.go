package wsw

// CaptureOptions describes an Instant Snap capture. Only URL is required; every field left
// unset is not sent and takes the API's default (see [InstantSnapService.Defaults]).
type CaptureOptions struct {
	// URL is the page to capture: absolute http or https, without credentials.
	URL string `json:"url"`

	// MediaCategory defaults to image.
	MediaCategory MediaCategory `json:"media_category,omitempty"`
	// Format must belong to MediaCategory. image: png, jpeg, webp, tiff; video: mp4, webm,
	// mov, gif; pdf: image_pdf, document_pdf.
	Format Format `json:"format,omitempty"`

	Viewport *Viewport `json:"viewport,omitempty"`
	// DeviceCategory defaults to desktop.
	DeviceCategory DeviceCategory `json:"device_category,omitempty"`
	// EmulatedDevice is a device from [InstantSnapService.Catalog], matched without regard
	// to case. It defaults to "iPhone 13" when DeviceCategory is mobile.
	EmulatedDevice *string `json:"emulated_device,omitempty"`

	// ScrollMode defaults to viewport for images and to scroll for video.
	ScrollMode ScrollMode `json:"scroll_mode,omitempty"`
	AutoHeight *bool      `json:"auto_height,omitempty"`
	// MaxHeight is refused, not lowered, above the format's ceiling (WebP 16383 divided by the
	// scale factor, JPEG 65500).
	MaxHeight *int `json:"max_height,omitempty"`
	// Zoom is a percentage from 50 to 100.
	Zoom *int `json:"zoom,omitempty"`

	BlockAds    *bool `json:"block_ads,omitempty"`
	BlockPopUps *bool `json:"block_pop_ups,omitempty"`
	// PressEscape presses Escape after the pop-up pass and again after scrolling (once in a
	// video), for a modal that closes on Escape. The default is false.
	PressEscape *bool `json:"press_escape,omitempty"`

	PageTimeoutMS *int `json:"page_timeout_ms,omitempty"`
	// ScrollingDelayMS is the pause after each scroll step of a full-page capture, from 0 to
	// 10000; the default is 500.
	ScrollingDelayMS *int `json:"scrolling_delay_ms,omitempty"`

	UserAgent *string `json:"user_agent,omitempty"`
	// Locale is a BCP 47 language tag.
	Locale *string `json:"locale,omitempty"`
	// TimeZone is an IANA time zone name.
	TimeZone *string `json:"time_zone,omitempty"`

	// Headers are sent with every request the page makes, with their names exactly as given.
	// They need the RequestHeadersCookies plan feature.
	Headers map[string]string `json:"headers,omitempty"`
	// Cookies are set for the URL's origin, with their names exactly as given. They need the
	// RequestHeadersCookies plan feature.
	Cookies map[string]string `json:"cookies,omitempty"`

	Geolocation        *Geolocation        `json:"geolocation,omitempty"`
	Selectors          *Selectors          `json:"selectors,omitempty"`
	Proxy              *Proxy              `json:"proxy,omitempty"`
	TimestampWatermark *TimestampWatermark `json:"timestamp_watermark,omitempty"`
	PageSource         *PageSourceOptions  `json:"page_source,omitempty"`
	// S3Delivery is where [InstantSnapService.CaptureToBucket] and
	// [InstantSnapService.SubmitToBucket] upload the file. The other capture methods refuse it.
	S3Delivery *S3DeliveryOptions `json:"s3_delivery,omitempty"`

	// Timeout is the browser time, in seconds, an async capture may use: async only
	// ([InstantSnapService.Submit] and [InstantSnapService.SubmitToBucket]), from 30 to 300;
	// the server default is 120. The server refuses it on the other capture methods.
	Timeout *int `json:"timeout,omitempty"`

	// CallbackURL also receives the capture.succeeded or capture.failed webhook. It needs
	// the Webhooks plan feature.
	CallbackURL *string `json:"callback_url,omitempty"`
}

// Viewport sets the browser window, in CSS pixels.
type Viewport struct {
	// Width is from 250 to 7680; the default is 1920.
	Width *int `json:"width,omitempty"`
	// Height is from 250 to 7680; the default is 1080.
	Height *int `json:"height,omitempty"`
	// DeviceScaleFactor is the device pixel ratio, from 1 to 3; the default is 1.
	DeviceScaleFactor *float64 `json:"device_scale_factor,omitempty"`
}

// Geolocation is the position the page sees. Latitude and longitude are sent together or not
// at all.
type Geolocation struct {
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	// Accuracy is in metres, from 1 to 10000; the default is 100.
	Accuracy *float64 `json:"accuracy,omitempty"`
}

// Selectors act on parts of the page before it is captured.
type Selectors struct {
	// XPath limits the capture to the element it selects. At most 512 characters.
	XPath *string `json:"xpath,omitempty"`
	// Hide is up to 25 comma-separated CSS selectors (an XPath starting with // works too) of
	// elements to hide, at most 2000 characters. Each is applied on its own; one the browser
	// rejects is skipped.
	Hide *string `json:"hide,omitempty"`
	// Click is a CSS selector of an element to click. At most 512 characters.
	Click *string `json:"click,omitempty"`
	// Hover is a CSS selector of an element to hover over. At most 512 characters.
	Hover *string `json:"hover,omitempty"`
}

// Proxy sends the capture through a proxy in another country. It needs the ProxyLocation plan
// feature.
type Proxy struct {
	// Country is one of the proxy countries in [InstantSnapService.Catalog].
	Country string `json:"country"`
}

// TimestampWatermark adds a strip with the capture time and URL. Sending the block, even
// empty, turns it on. It is not available for video.
type TimestampWatermark struct {
	Enabled  *bool             `json:"enabled,omitempty"`
	Location WatermarkLocation `json:"location,omitempty"`
	// FontColor is a hex colour; the default is #000000.
	FontColor *string `json:"font_color,omitempty"`
	// BackgroundColor is a hex colour; the default is #FFFFFF.
	BackgroundColor *string `json:"background_color,omitempty"`
}

// PageSourceOptions also keeps the page source, capped at 10 MB and charged as a second
// capture. It is not available with [InstantSnapService.CaptureFile].
type PageSourceOptions struct {
	// Format defaults to html.
	Format PageSourceFormat `json:"format,omitempty"`
}

// S3DeliveryOptions is an S3-compatible bucket to upload the capture to. It needs Region or
// EndpointURL.
type S3DeliveryOptions struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	Bucket          string `json:"bucket"`

	Region *string `json:"region,omitempty"`
	// EndpointURL is for providers other than AWS: https only, and not a private address.
	EndpointURL    *string `json:"endpoint_url,omitempty"`
	ForcePathStyle *bool   `json:"force_path_style,omitempty"`
	KeyPrefix      *string `json:"key_prefix,omitempty"`
	// WaitForSync waits up to 30 seconds for the upload to finish before answering.
	WaitForSync *bool `json:"wait_for_sync,omitempty"`
}
