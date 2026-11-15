package wsw_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/websitescreenshotworld/websitescreenshotworld-go"
)

type renderLinkVectors struct {
	Cases []struct {
		Name         string                     `json:"name"`
		RenderSecret string                     `json:"render_secret"`
		KeyPrefix    string                     `json:"key_prefix"`
		Format       string                     `json:"format"`
		BaseURL      string                     `json:"base_url"`
		Options      map[string]json.RawMessage `json:"options"`
		URL          string                     `json:"url"`
	} `json:"cases"`
}

func TestRenderLinkVectors(t *testing.T) {
	var vectors renderLinkVectors
	readJSON(t, "spec/render-link-vectors.json", &vectors)
	if len(vectors.Cases) == 0 {
		t.Fatal("no cases")
	}

	for _, tc := range vectors.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			o := wsw.RenderLinkOptions{
				RenderSecret: tc.RenderSecret,
				KeyPrefix:    tc.KeyPrefix,
				Format:       wsw.Format(tc.Format),
				BaseURL:      tc.BaseURL,
			}
			for name, raw := range tc.Options {
				applyRenderLinkOption(t, &o, name, raw)
			}

			got, err := wsw.RenderLink(o)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.URL {
				t.Fatalf("got  %s\nwant %s", got, tc.URL)
			}
		})
	}
}

func TestRenderLinkRefusesWhatCannotBeALink(t *testing.T) {
	valid := wsw.RenderLinkOptions{RenderSecret: "wsw_rsk_x", KeyPrefix: "0123456789ab", URL: "https://example.com"}
	link, err := wsw.RenderLink(valid)
	if err != nil || !regexp.MustCompile(`&sig=[0-9a-f]{64}$`).MatchString(link) {
		t.Fatalf("got %q, %v", link, err)
	}

	for name, change := range map[string]func(*wsw.RenderLinkOptions){
		"no secret":         func(o *wsw.RenderLinkOptions) { o.RenderSecret = "" },
		"upper-case prefix": func(o *wsw.RenderLinkOptions) { o.KeyPrefix = "0123456789AB" },
		"whole key":         func(o *wsw.RenderLinkOptions) { o.KeyPrefix = "wsw_api_0123" },
		"no url":            func(o *wsw.RenderLinkOptions) { o.URL = "" },
		"pdf":               func(o *wsw.RenderLinkOptions) { o.Format = wsw.FormatImagePDF },
	} {
		o := valid
		change(&o)
		if _, err := wsw.RenderLink(o); err == nil {
			t.Errorf("%s: got no error", name)
		}
	}

	o := valid
	o.BlockAds = wsw.Bool(false)
	link, _ = wsw.RenderLink(o)
	if !strings.Contains(link, "block_ads=false") || strings.Contains(link, "width=") {
		t.Fatalf("got %s", link)
	}
}

func applyRenderLinkOption(t *testing.T, o *wsw.RenderLinkOptions, name string, raw json.RawMessage) {
	t.Helper()
	var s string
	var i int
	var f float64
	var b bool
	unmarshal := func(v any) {
		if err := json.Unmarshal(raw, v); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	switch name {
	case "url":
		unmarshal(&s)
		o.URL = s
	case "ttl":
		unmarshal(&i)
		o.TTLSeconds = wsw.Int(i)
	case "width":
		unmarshal(&i)
		o.Width = wsw.Int(i)
	case "height":
		unmarshal(&i)
		o.Height = wsw.Int(i)
	case "device_scale_factor":
		unmarshal(&f)
		o.DeviceScaleFactor = wsw.Float64(f)
	case "device_category":
		unmarshal(&s)
		o.DeviceCategory = wsw.DeviceCategory(s)
	case "emulated_device":
		unmarshal(&s)
		o.EmulatedDevice = wsw.String(s)
	case "scroll_mode":
		unmarshal(&s)
		o.ScrollMode = wsw.ScrollMode(s)
	case "auto_height":
		unmarshal(&b)
		o.AutoHeight = wsw.Bool(b)
	case "max_height":
		unmarshal(&i)
		o.MaxHeight = wsw.Int(i)
	case "zoom":
		unmarshal(&i)
		o.Zoom = wsw.Int(i)
	case "block_ads":
		unmarshal(&b)
		o.BlockAds = wsw.Bool(b)
	case "block_pop_ups":
		unmarshal(&b)
		o.BlockPopUps = wsw.Bool(b)
	case "press_escape":
		unmarshal(&b)
		o.PressEscape = wsw.Bool(b)
	case "page_timeout_ms":
		unmarshal(&i)
		o.PageTimeoutMS = wsw.Int(i)
	case "scrolling_delay_ms":
		unmarshal(&i)
		o.ScrollingDelayMS = wsw.Int(i)
	case "locale":
		unmarshal(&s)
		o.Locale = wsw.String(s)
	case "time_zone":
		unmarshal(&s)
		o.TimeZone = wsw.String(s)
	case "hide":
		unmarshal(&s)
		o.Hide = wsw.String(s)
	case "xpath":
		unmarshal(&s)
		o.XPath = wsw.String(s)
	case "click":
		unmarshal(&s)
		o.Click = wsw.String(s)
	case "hover":
		unmarshal(&s)
		o.Hover = wsw.String(s)
	case "country":
		unmarshal(&s)
		o.Country = wsw.String(s)
	case "thumb_width":
		unmarshal(&i)
		o.ThumbWidth = wsw.Int(i)
	default:
		t.Fatalf("the vector names an option this test does not know: %s", name)
	}
}
