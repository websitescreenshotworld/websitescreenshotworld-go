package wsw

// Defaults are the values a capture takes for the options it does not send, and the limits it
// is checked against. They are versioned by SpecVersion.
type Defaults struct {
	SpecVersion   string        `json:"spec_version"`
	ResponseType  ResponseType  `json:"response_type"`
	MediaCategory MediaCategory `json:"media_category"`
	// Format is the default format of each media category.
	Format map[MediaCategory]Format `json:"format"`
	// Formats lists the formats each media category accepts.
	Formats        map[MediaCategory][]Format `json:"formats"`
	DeviceCategory DeviceCategory             `json:"device_category"`
	EmulatedDevice string                     `json:"emulated_device"`
	Viewport       ViewportDefaults           `json:"viewport"`
	Zoom           int                        `json:"zoom"`
	ScrollMode     ScrollMode                 `json:"scroll_mode"`
	AutoHeight     bool                       `json:"auto_height"`
	// AutoHeightMaxHeight is zero when there is no general ceiling.
	AutoHeightMaxHeight     int     `json:"auto_height_max_height"`
	AutoHeightMaxHeightWebP int     `json:"auto_height_max_height_webp"`
	BlockAds                bool    `json:"block_ads"`
	BlockPopUps             bool    `json:"block_pop_ups"`
	PressEscape             bool    `json:"press_escape"`
	PageTimeoutMS           int     `json:"page_timeout_ms"`
	ScrollingDelayMS        int     `json:"scrolling_delay_ms"`
	Geolocation             bool    `json:"geolocation"`
	GeolocationAccuracy     float64 `json:"geolocation_accuracy"`
	WaitForCloudSync        bool    `json:"wait_for_cloud_sync"`
	// TimestampWatermark, PageSource and Limits are passed through as the API sends them.
	TimestampWatermark map[string]any `json:"timestamp_watermark"`
	PageSource         map[string]any `json:"page_source"`
	Limits             map[string]any `json:"limits"`
	// LeftToTheBrowser names the settings the API does not set, leaving the browser's own.
	LeftToTheBrowser []string `json:"left_to_the_browser"`
}

// ViewportDefaults is the default viewport.
type ViewportDefaults struct {
	Width             int     `json:"width"`
	Height            int     `json:"height"`
	DeviceScaleFactor float64 `json:"device_scale_factor"`
}

// Catalog lists what [CaptureOptions.EmulatedDevice] and [Proxy.Country] accept.
type Catalog struct {
	EmulatedDevices []string `json:"emulated_devices"`
	ProxyCountries  []string `json:"proxy_countries"`
}
