package wsw_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/websitescreenshotworld/websitescreenshotworld-go"
)

func ExampleNewClient() {
	// Keys default to WSW_INSTANT_SNAP_API_KEY and WSW_AUTO_SCHEDULE_API_KEY.
	client := wsw.NewClient(
		wsw.WithInstantSnapKey(os.Getenv("MY_INSTANT_SNAP_KEY")),
		wsw.WithMaxRetries(3),
		wsw.WithCaptureTimeout(4*time.Minute),
	)
	_ = client
}

func ExampleInstantSnapService_Capture() {
	client := wsw.NewClient()
	ctx := context.Background()

	capture, err := client.InstantSnap.Capture(ctx, &wsw.CaptureOptions{
		URL:        "https://example.com",
		Format:     wsw.FormatWebP,
		ScrollMode: wsw.ScrollModeScroll,
		Viewport:   &wsw.Viewport{Width: wsw.Int(1440), Height: wsw.Int(900)},
		BlockAds:   wsw.Bool(false),
		Headers:    map[string]string{"X-Preview": "1"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(capture.ID, capture.Width, capture.Height)

	if err := client.InstantSnap.DownloadToFile(ctx, capture, "example.webp"); err != nil {
		log.Fatal(err)
	}
}

func ExampleInstantSnapService_CaptureFile() {
	client := wsw.NewClient()

	file, err := client.InstantSnap.CaptureFile(context.Background(), &wsw.CaptureOptions{
		URL:           "https://example.com",
		MediaCategory: wsw.MediaCategoryPDF,
		Format:        wsw.FormatDocumentPDF,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := file.SaveTo(file.Filename); err != nil {
		log.Fatal(err)
	}
}

func ExampleInstantSnapService_CaptureToBucket() {
	client := wsw.NewClient()

	capture, err := client.InstantSnap.CaptureToBucket(context.Background(), &wsw.CaptureOptions{
		URL: "https://example.com",
		S3Delivery: &wsw.S3DeliveryOptions{
			AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
			Bucket:          "captures",
			Region:          wsw.String("eu-west-2"),
			KeyPrefix:       wsw.String("homepage/"),
		},
	}, wsw.WithIdempotencyKey("homepage-2026-09-26"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(capture.S3Delivery.Status)
}

func ExampleInstantSnapService_Submit() {
	client := wsw.NewClient()
	ctx := context.Background()

	queued, err := client.InstantSnap.Submit(ctx, &wsw.CaptureOptions{
		URL:     "https://example.com",
		Timeout: wsw.Int(240),
	})
	if err != nil {
		log.Fatal(err)
	}

	capture, err := client.InstantSnap.Wait(ctx, queued.ID)
	if err != nil {
		log.Fatal(err)
	}
	switch capture.Status {
	case wsw.CaptureStatusSucceeded:
		fmt.Println(capture.DownloadURL)
	case wsw.CaptureStatusQueued, wsw.CaptureStatusRunning:
		fmt.Println("still running; wait again later")
	default:
		fmt.Println("ended", capture.Status, capture.ErrorCode)
	}
}

func ExampleAutoScheduleService_TriggerNow() {
	client := wsw.NewClient()

	result, err := client.AutoSchedule.TriggerNow(context.Background(), "7d4e2b1a-6c5f-4a3e-9d8c-2b1a0f9e8d7c")
	if err != nil {
		log.Fatal(err)
	}
	if result.Status == wsw.TriggerStatusAlreadyPending {
		fmt.Println("already queued at", result.TriggeredAt)
	}
}

func ExampleError() {
	client := wsw.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

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
		log.Printf("%s (%d): %s", apiErr.Code, apiErr.HTTPStatus, apiErr.Message)
	default:
		log.Print(err)
	}
}

func ExampleVerifyWebhook() {
	secret := os.Getenv("WSW_WEBHOOK_SECRET")

	http.HandleFunc("/webhooks/wsw", func(w http.ResponseWriter, r *http.Request) {
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
		case *wsw.AutoScheduleCaptureFailedEvent:
			log.Printf("%s failed with HTTP %d", e.Data.ScheduleName, e.Data.PageHTTPStatus)
		case *wsw.UnknownEvent:
			log.Printf("ignoring %s", e.Type)
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
