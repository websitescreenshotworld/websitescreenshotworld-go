package wsw

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// CaptureFile is the answer of [InstantSnapService.CaptureFile]: the file as a stream, and its
// metadata. Read Body once and close it, or call SaveTo.
type CaptureFile struct {
	// Body streams the file. The capture timeout keeps running until it is closed.
	Body io.ReadCloser

	ID     string
	Status CaptureStatus
	// ContentType is the file's media type, such as "image/png".
	ContentType string
	// Filename is the name the API suggests, from Content-Disposition.
	Filename   string
	Width      int
	Height     int
	SizeBytes  int64
	DurationMS int
	// PageHTTPStatus is the status the website answered with.
	PageHTTPStatus int
	SpecVersion    string
	CapturedAt     time.Time
	// FinalURL is where the browser ended up. It is empty when the URL is not plain ASCII.
	FinalURL string
	// SHA256 is the lowercase hex digest of the file.
	SHA256 string
	// PartCount is the number of parts in the answer, 1 unless the API says otherwise.
	PartCount int
}

func newCaptureFile(resp *http.Response) *CaptureFile {
	h := resp.Header
	f := &CaptureFile{
		Body:           resp.Body,
		ID:             h.Get("X-InstantSnap-Id"),
		Status:         CaptureStatus(h.Get("X-InstantSnap-Status")),
		ContentType:    h.Get("Content-Type"),
		Width:          headerInt(h, "X-InstantSnap-Width"),
		Height:         headerInt(h, "X-InstantSnap-Height"),
		SizeBytes:      int64(headerInt(h, "X-InstantSnap-Size-Bytes")),
		DurationMS:     headerInt(h, "X-InstantSnap-Duration-Ms"),
		PageHTTPStatus: headerInt(h, "X-InstantSnap-Page-Http-Status"),
		SpecVersion:    h.Get("X-InstantSnap-Spec-Version"),
		FinalURL:       h.Get("X-InstantSnap-Final-Url"),
		SHA256:         h.Get("X-InstantSnap-Sha256"),
		PartCount:      max(headerInt(h, "X-InstantSnap-Part-Count"), 1),
	}
	if t, err := time.Parse(time.RFC3339, h.Get("X-InstantSnap-Captured-At")); err == nil {
		f.CapturedAt = t
	}
	if _, params, err := mime.ParseMediaType(h.Get("Content-Disposition")); err == nil {
		f.Filename = filepath.Base(params["filename"])
		if f.Filename == "." || f.Filename == string(filepath.Separator) {
			f.Filename = ""
		}
	}
	return f
}

// Close closes Body.
func (f *CaptureFile) Close() error {
	return f.Body.Close()
}

// SaveTo writes the file to path, replacing any file there, and closes Body. Nothing is left at
// path if writing fails.
func (f *CaptureFile) SaveTo(path string) error {
	defer f.Body.Close()
	return writeFile(path, func(w io.Writer) error {
		if _, err := io.Copy(w, f.Body); err != nil {
			return fmt.Errorf("wsw: reading capture %s: %w", f.ID, err)
		}
		return nil
	})
}

func headerInt(h http.Header, name string) int {
	n, err := strconv.Atoi(h.Get(name))
	if err != nil {
		return 0
	}
	return n
}

// writeFile writes to a temporary file beside path and renames it into place, so a failed
// write never leaves a partial file behind or clobbers an existing one.
func writeFile(path string, write func(io.Writer) error) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("wsw: %w", err)
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if err = write(tmp); err != nil {
		return err
	}
	// CreateTemp makes the file readable by its owner only; a saved capture is an ordinary file.
	if err = tmp.Chmod(0o644); err != nil {
		return fmt.Errorf("wsw: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("wsw: %w", err)
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("wsw: %w", err)
	}
	return nil
}
