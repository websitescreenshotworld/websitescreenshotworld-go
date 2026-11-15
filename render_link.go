package wsw

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"
)

// RenderLinkOptions is what a render link captures. RenderSecret, KeyPrefix and URL are
// required; an option left nil (or empty, for the typed tokens) is not in the link, and the
// API applies its default.
type RenderLinkOptions struct {
	// RenderSecret is the key's render secret (wsw_rsk_…), shown on the API Keys page once
	// render links are on for the key.
	RenderSecret string
	// KeyPrefix is the key's 12-character prefix: the part after wsw_api_ and before the next _.
	KeyPrefix string
	// URL is the page to capture.
	URL string
	// Format is png (the default), jpeg or webp.
	Format Format
	// BaseURL defaults to DefaultInstantSnapBaseURL.
	BaseURL string
	// TTLSeconds is how long a capture of the link is served from cache before a view
	// captures again, 3600 to 2592000. The server uses 86400 (a day) when it is nil.
	TTLSeconds *int

	Width             *int
	Height            *int
	DeviceScaleFactor *float64
	DeviceCategory    DeviceCategory
	EmulatedDevice    *string
	ScrollMode        ScrollMode
	AutoHeight        *bool
	MaxHeight         *int
	Zoom              *int
	BlockAds          *bool
	BlockPopUps       *bool
	PressEscape       *bool
	PageTimeoutMS     *int
	ScrollingDelayMS  *int
	Locale            *string
	TimeZone          *string
	// Hide, XPath, Click and Hover are the capture's selectors.
	Hide  *string
	XPath *string
	Click *string
	Hover *string
	// Country is the capture's proxy country.
	Country *string
	// ThumbWidth shrinks the image to this width after the capture (16 to 3840), keeping its
	// proportions.
	ThumbWidth *int
}

const renderLinkPathPrefix = "/instant-snap/v1/render/"

// RenderLink builds a render link: a signed URL that is itself a screenshot, for an <img>
// tag, an email or a social preview. It is made here, locally - nothing is sent, and no client
// or API key is needed. Every view of the link that is not already cached makes one Instant
// Snap capture.
//
// It returns an error when RenderSecret, KeyPrefix or URL is empty, KeyPrefix is not 12
// lower-case hex characters, or Format is not png, jpeg or webp.
func RenderLink(o RenderLinkOptions) (string, error) {
	if o.RenderSecret == "" {
		return "", errors.New("wsw: render secret is empty")
	}
	if !isKeyPrefix(o.KeyPrefix) {
		return "", errors.New("wsw: key prefix must be the key's 12-character prefix (lower-case hex)")
	}
	if o.URL == "" {
		return "", errors.New("wsw: url is empty")
	}
	format := o.Format
	if format == "" {
		format = FormatPNG
	}
	if format != FormatPNG && format != FormatJPEG && format != FormatWebP {
		return "", errors.New("wsw: format must be png, jpeg or webp")
	}

	var pairs [][2]string
	add := func(name, value string) {
		pairs = append(pairs, [2]string{rfc3986(name), rfc3986(value)})
	}
	addInt := func(name string, v *int) {
		if v != nil {
			add(name, strconv.Itoa(*v))
		}
	}
	addBool := func(name string, v *bool) {
		if v != nil {
			add(name, strconv.FormatBool(*v))
		}
	}
	addString := func(name string, v *string) {
		if v != nil {
			add(name, *v)
		}
	}

	add("url", o.URL)
	addInt("ttl", o.TTLSeconds)
	addInt("width", o.Width)
	addInt("height", o.Height)
	if o.DeviceScaleFactor != nil {
		add("device_scale_factor", strconv.FormatFloat(*o.DeviceScaleFactor, 'f', -1, 64))
	}
	if o.DeviceCategory != "" {
		add("device_category", string(o.DeviceCategory))
	}
	addString("emulated_device", o.EmulatedDevice)
	if o.ScrollMode != "" {
		add("scroll_mode", string(o.ScrollMode))
	}
	addBool("auto_height", o.AutoHeight)
	addInt("max_height", o.MaxHeight)
	addInt("zoom", o.Zoom)
	addBool("block_ads", o.BlockAds)
	addBool("block_pop_ups", o.BlockPopUps)
	addBool("press_escape", o.PressEscape)
	addInt("page_timeout_ms", o.PageTimeoutMS)
	addInt("scrolling_delay_ms", o.ScrollingDelayMS)
	addString("locale", o.Locale)
	addString("time_zone", o.TimeZone)
	addString("hide", o.Hide)
	addString("xpath", o.XPath)
	addString("click", o.Click)
	addString("hover", o.Hover)
	addString("country", o.Country)
	addInt("thumb_width", o.ThumbWidth)

	sort.Slice(pairs, func(i, j int) bool { return pairs[i][0] < pairs[j][0] })

	var canonical strings.Builder
	canonical.WriteString(renderLinkPathPrefix + o.KeyPrefix + "/" + string(format) + "?")
	for i, pair := range pairs {
		if i > 0 {
			canonical.WriteByte('&')
		}
		canonical.WriteString(pair[0] + "=" + pair[1])
	}

	mac := hmac.New(sha256.New, []byte(o.RenderSecret))
	mac.Write([]byte(canonical.String()))
	signature := hex.EncodeToString(mac.Sum(nil))

	baseURL := o.BaseURL
	if baseURL == "" {
		baseURL = DefaultInstantSnapBaseURL
	}
	return strings.TrimRight(baseURL, "/") + canonical.String() + "&sig=" + signature, nil
}

func isKeyPrefix(value string) bool {
	if len(value) != 12 {
		return false
	}
	for _, c := range value {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// rfc3986 percent-encodes every UTF-8 byte but A-Z a-z 0-9 - . _ ~, with upper-case hex.
func rfc3986(value string) string {
	const hexDigits = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
			c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hexDigits[c>>4])
		b.WriteByte(hexDigits[c&15])
	}
	return b.String()
}
