package wsw_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/websitescreenshotworld/websitescreenshotworld-go"
)

type fixture struct {
	Name string `json:"name"`
	Call struct {
		Product        string          `json:"product"`
		Method         string          `json:"method"`
		Options        json.RawMessage `json:"options"`
		IdempotencyKey string          `json:"idempotency_key"`
		ID             string          `json:"id"`
		WebPageID      string          `json:"web_page_id"`
		// PollIntervalSeconds and TimeoutSeconds are wait's arguments.
		PollIntervalSeconds *float64 `json:"poll_interval_seconds"`
		TimeoutSeconds      *float64 `json:"timeout_seconds"`
	} `json:"call"`
	ExpectRequests []struct {
		Method         string            `json:"method"`
		Path           string            `json:"path"`
		Headers        map[string]string `json:"headers"`
		IdempotencyKey string            `json:"idempotency_key"`
		Body           json.RawMessage   `json:"body"`
	} `json:"expect_requests"`
	Expect struct {
		Result map[string]any `json:"result"`
		Error  *struct {
			Kind              string   `json:"kind"`
			ErrorCode         string   `json:"error_code"`
			HTTPStatus        int      `json:"http_status"`
			Message           *string  `json:"message"`
			RetryAfterSeconds *float64 `json:"retry_after_seconds"`
		} `json:"error"`
	} `json:"expect"`
}

type recordedRequest struct {
	Method  string         `json:"method"`
	Path    string         `json:"path"`
	Headers map[string]any `json:"headers"`
	Body    *string        `json:"body"`
}

var sentinelByKind = map[string]error{
	"authentication":  wsw.ErrAuthentication,
	"invalid_request": wsw.ErrInvalidRequest,
	"plan_limit":      wsw.ErrPlanLimit,
	"not_found":       wsw.ErrNotFound,
	"rate_limit":      wsw.ErrRateLimit,
	"capture":         wsw.ErrCapture,
	"service":         wsw.ErrService,
}

func TestConformance(t *testing.T) {
	base := startConformanceServer(t)

	var keys struct {
		InstantSnap  string `json:"instant_snap"`
		AutoSchedule string `json:"auto_schedule"`
	}
	readJSON(t, "spec/fixtures/keys.json", &keys)

	files, err := filepath.Glob("spec/fixtures/*.json")
	if err != nil {
		t.Fatal(err)
	}
	ran := 0
	for _, file := range files {
		if filepath.Base(file) == "keys.json" {
			continue
		}
		ran++
		t.Run(strings.TrimSuffix(filepath.Base(file), ".json"), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var fx fixture
			if err := json.Unmarshal(raw, &fx); err != nil {
				t.Fatal(err)
			}
			post(t, base+"/__load", raw)

			client := wsw.NewClient(
				wsw.WithInstantSnapKey(keys.InstantSnap),
				wsw.WithAutoScheduleKey(keys.AutoSchedule),
				wsw.WithBaseURLs(base, base),
			)
			result, callErr := runFixtureCall(t, client, &fx)

			var sent []recordedRequest
			getJSON(t, base+"/__requests", &sent)
			checkRequests(t, &fx, sent)

			if fx.Expect.Error != nil {
				checkError(t, &fx, callErr)
				return
			}
			if callErr != nil {
				t.Fatalf("call failed: %v", callErr)
			}
			matchSubset(t, "result", fx.Expect.Result, result)
		})
	}
	if ran == 0 {
		t.Fatal("no fixtures found")
	}
}

// runFixtureCall makes the fixture's call and returns its result in wire form.
func runFixtureCall(t *testing.T, client *wsw.Client, fx *fixture) (map[string]any, error) {
	t.Helper()
	ctx := context.Background()

	var opts *wsw.CaptureOptions
	if len(fx.Call.Options) > 0 {
		opts = new(wsw.CaptureOptions)
		if err := json.Unmarshal(fx.Call.Options, opts); err != nil {
			t.Fatalf("reading options: %v", err)
		}
	}
	var reqOpts []wsw.RequestOption
	if fx.Call.IdempotencyKey != "" {
		reqOpts = append(reqOpts, wsw.WithIdempotencyKey(fx.Call.IdempotencyKey))
	}

	var (
		result any
		err    error
	)
	switch fx.Call.Product + "." + fx.Call.Method {
	case "instant_snap.capture":
		result, err = client.InstantSnap.Capture(ctx, opts, reqOpts...)
	case "instant_snap.capture_to_bucket":
		result, err = client.InstantSnap.CaptureToBucket(ctx, opts, reqOpts...)
	case "instant_snap.submit":
		result, err = client.InstantSnap.Submit(ctx, opts, reqOpts...)
	case "instant_snap.submit_to_bucket":
		result, err = client.InstantSnap.SubmitToBucket(ctx, opts, reqOpts...)
	case "instant_snap.wait":
		var waitOpts []wsw.WaitOption
		if s := fx.Call.PollIntervalSeconds; s != nil {
			waitOpts = append(waitOpts, wsw.WithPollInterval(time.Duration(*s*float64(time.Second))))
		}
		if s := fx.Call.TimeoutSeconds; s != nil {
			waitOpts = append(waitOpts, wsw.WithWaitTimeout(time.Duration(*s*float64(time.Second))))
		}
		result, err = client.InstantSnap.Wait(ctx, fx.Call.ID, waitOpts...)
	case "instant_snap.capture_file":
		var file *wsw.CaptureFile
		file, err = client.InstantSnap.CaptureFile(ctx, opts, reqOpts...)
		if err != nil {
			return nil, err
		}
		return captureFileResult(t, file), nil
	case "instant_snap.get":
		result, err = client.InstantSnap.Get(ctx, fx.Call.ID)
	case "instant_snap.defaults":
		result, err = client.InstantSnap.Defaults(ctx)
	case "instant_snap.catalog":
		result, err = client.InstantSnap.Catalog(ctx)
	case "auto_schedule.trigger_now":
		result, err = client.AutoSchedule.TriggerNow(ctx, fx.Call.WebPageID)
	default:
		t.Fatalf("unknown call %s.%s", fx.Call.Product, fx.Call.Method)
	}
	if err != nil {
		return nil, err
	}
	return toWire(t, result), nil
}

func captureFileResult(t *testing.T, f *wsw.CaptureFile) map[string]any {
	t.Helper()
	defer f.Close()
	data, err := io.ReadAll(f.Body)
	if err != nil {
		t.Fatalf("reading the file: %v", err)
	}
	sum := sha256.Sum256(data)
	return toWire(t, map[string]any{
		"id":               f.ID,
		"status":           f.Status,
		"content_type":     f.ContentType,
		"filename":         f.Filename,
		"width":            f.Width,
		"height":           f.Height,
		"size_bytes":       f.SizeBytes,
		"duration_ms":      f.DurationMS,
		"page_http_status": f.PageHTTPStatus,
		"spec_version":     f.SpecVersion,
		"captured_at":      f.CapturedAt,
		"final_url":        f.FinalURL,
		"sha256":           f.SHA256,
		"body_sha256":      hex.EncodeToString(sum[:]),
	})
}

func checkRequests(t *testing.T, fx *fixture, sent []recordedRequest) {
	t.Helper()
	if len(sent) != len(fx.ExpectRequests) {
		t.Fatalf("sent %d requests, want %d", len(sent), len(fx.ExpectRequests))
	}
	var sharedKey string
	for i, want := range fx.ExpectRequests {
		got := sent[i]
		if got.Method != want.Method || got.Path != want.Path {
			t.Errorf("request %d: %s %s, want %s %s", i, got.Method, got.Path, want.Method, want.Path)
		}
		for name, value := range want.Headers {
			if h, _ := got.Headers[strings.ToLower(name)].(string); h != value {
				t.Errorf("request %d: header %s = %q, want %q", i, name, h, value)
			}
		}

		key, _ := got.Headers["idempotency-key"].(string)
		switch want.IdempotencyKey {
		case "present":
			if key == "" {
				t.Errorf("request %d: no Idempotency-Key", i)
			}
		case "same":
			if key == "" {
				t.Errorf("request %d: no Idempotency-Key", i)
			}
			if i == 0 {
				sharedKey = key
			} else if key != sharedKey {
				t.Errorf("request %d: Idempotency-Key %q, want %q as before", i, key, sharedKey)
			}
		}

		if len(want.Body) == 0 || string(want.Body) == "null" {
			if got.Body != nil {
				t.Errorf("request %d: body %s, want none", i, *got.Body)
			}
			continue
		}
		if got.Body == nil {
			t.Errorf("request %d: no body, want %s", i, want.Body)
			continue
		}
		var wantBody, gotBody any
		if err := json.Unmarshal(want.Body, &wantBody); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(*got.Body), &gotBody); err != nil {
			t.Fatalf("request %d: body is not JSON: %v", i, err)
		}
		if !reflect.DeepEqual(wantBody, gotBody) {
			t.Errorf("request %d: body\n got %s\nwant %s", i, *got.Body, want.Body)
		}
	}
}

func checkError(t *testing.T, fx *fixture, err error) {
	t.Helper()
	want := fx.Expect.Error
	var apiErr *wsw.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("got %v, want a *wsw.Error", err)
	}
	wantKind := wsw.ErrorKind(want.Kind)
	if want.Kind == "base" {
		wantKind = wsw.KindUnknown
	}
	if apiErr.Kind != wantKind {
		t.Errorf("Kind = %q, want %q", apiErr.Kind, wantKind)
	}
	if sentinel, ok := sentinelByKind[want.Kind]; ok && !errors.Is(err, sentinel) {
		t.Errorf("errors.Is(err, %v) = false", sentinel)
	}
	if apiErr.Code != want.ErrorCode {
		t.Errorf("Code = %q, want %q", apiErr.Code, want.ErrorCode)
	}
	if apiErr.HTTPStatus != want.HTTPStatus {
		t.Errorf("HTTPStatus = %d, want %d", apiErr.HTTPStatus, want.HTTPStatus)
	}
	if want.Message != nil && apiErr.Message != *want.Message {
		t.Errorf("Message = %q, want %q", apiErr.Message, *want.Message)
	}
	if want.RetryAfterSeconds != nil {
		if wantWait := time.Duration(*want.RetryAfterSeconds * float64(time.Second)); apiErr.RetryAfter != wantWait {
			t.Errorf("RetryAfter = %v, want %v", apiErr.RetryAfter, wantWait)
		}
	}
	if len(apiErr.Body) == 0 {
		t.Error("Body is empty")
	}
}

// matchSubset checks that every field of want is in got with the same value.
func matchSubset(t *testing.T, path string, want, got any) {
	t.Helper()
	wantMap, ok := want.(map[string]any)
	if !ok {
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s = %#v, want %#v", path, got, want)
		}
		return
	}
	gotMap, ok := got.(map[string]any)
	if !ok {
		t.Errorf("%s = %#v, want an object", path, got)
		return
	}
	for k, v := range wantMap {
		gv, present := gotMap[k]
		if !present {
			t.Errorf("%s.%s is missing", path, k)
			continue
		}
		matchSubset(t, path+"."+k, v, gv)
	}
}

func startConformanceServer(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal("node is needed to run the conformance tests")
	}
	cmd := exec.Command("node", "spec/conformance/server.js")
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("reading the server's address: %v", err)
	}
	addr, ok := strings.CutPrefix(strings.TrimSpace(line), "listening on ")
	if !ok {
		t.Fatalf("unexpected first line from the server: %q", line)
	}
	return addr
}

func toWire(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func post(t *testing.T, url string, body []byte) {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s: %s", url, resp.Status)
	}
}

func getJSON(t *testing.T, url string, v any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatal(err)
	}
}
