package googleclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	gapi "google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type UploadOptions struct {
	Title         string
	Description   string
	PrivacyStatus string
}

type YouTubeService struct {
	svc *youtube.Service
}

func NewYouTubeService(ctx context.Context, httpClient *http.Client) (*YouTubeService, error) {
	svc, err := youtube.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create YouTube service: %w", err)
	}

	return &YouTubeService{svc: svc}, nil
}

func (y *YouTubeService) UploadVideo(ctx context.Context, filePath string, options UploadOptions) (*youtube.Video, error) {
	input, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open video file %q: %w", filePath, err)
	}
	defer input.Close()

	stat, err := input.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat video file %q: %w", filePath, err)
	}

	video := &youtube.Video{
		Snippet: &youtube.VideoSnippet{
			Title:       options.Title,
			Description: options.Description,
		},
		Status: &youtube.VideoStatus{
			PrivacyStatus: options.PrivacyStatus,
		},
	}

	reporter := newUploadReporter(stat.Size())
	trackedInput := &uploadReader{reader: input, reporter: reporter}
	call := y.svc.Videos.Insert([]string{"snippet", "status"}, video).
		Media(trackedInput, gapi.ChunkSize(8*1024*1024))

	result, err := call.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("upload video to YouTube: %w", err)
	}

	reporter.Finish()
	return result, nil
}

type uploadReporter struct {
	total   int64
	current int64
	lastLog time.Time
}

func newUploadReporter(total int64) *uploadReporter {
	return &uploadReporter{total: total}
}

func (r *uploadReporter) Update(current int64) {
	r.current = current
	if time.Since(r.lastLog) < time.Second && current < r.total {
		return
	}
	r.lastLog = time.Now()

	if r.total > 0 {
		percent := float64(current) / float64(r.total) * 100
		fmt.Printf("Uploading to YouTube: %.1f%% (%s/%s)\n", percent, humanBytes(current), humanBytes(r.total))
		return
	}

	fmt.Printf("Uploading to YouTube: %s\n", humanBytes(current))
}

func (r *uploadReporter) Finish() {
	if r.total > 0 {
		fmt.Printf("Uploading to YouTube: 100.0%% (%s/%s)\n", humanBytes(r.total), humanBytes(r.total))
	}
}

type uploadReader struct {
	reader   io.Reader
	reporter *uploadReporter
}

func (r *uploadReader) Read(buffer []byte) (int, error) {
	readBytes, err := r.reader.Read(buffer)
	if readBytes > 0 {
		r.reporter.Update(r.reporter.current + int64(readBytes))
	}
	return readBytes, err
}
