package googleclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type DriveFile struct {
	ID        string
	Name      string
	MimeType  string
	Size      int64
	CreatedAt time.Time
}

type DriveService struct {
	svc *drive.Service
}

func NewDriveService(ctx context.Context, httpClient *http.Client) (*DriveService, error) {
	svc, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create Google Drive service: %w", err)
	}

	return &DriveService{svc: svc}, nil
}

func (d *DriveService) ListVideosByDate(ctx context.Context, folderID string, day time.Time) ([]DriveFile, error) {
	query := fmt.Sprintf("'%s' in parents and trashed = false", folderID)
	call := d.svc.Files.List().
		Q(query).
		Fields("nextPageToken, files(id, name, mimeType, size, createdTime)").
		IncludeItemsFromAllDrives(true).
		SupportsAllDrives(true).
		PageSize(1000)

	var files []DriveFile
	for {
		response, err := call.Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("list Google Drive files: %w", err)
		}

		for _, item := range response.Files {
			if !strings.HasPrefix(item.MimeType, "video/") {
				continue
			}

			createdAt, err := time.Parse(time.RFC3339, item.CreatedTime)
			if err != nil {
				continue
			}

			createdAt = createdAt.Local()
			if !sameDay(createdAt, day) {
				continue
			}

			files = append(files, DriveFile{
				ID:        item.Id,
				Name:      item.Name,
				MimeType:  item.MimeType,
				Size:      item.Size,
				CreatedAt: createdAt,
			})
		}

		if response.NextPageToken == "" {
			break
		}
		call.PageToken(response.NextPageToken)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].CreatedAt.Before(files[j].CreatedAt)
	})

	return files, nil
}

func (d *DriveService) DownloadFile(ctx context.Context, file DriveFile, targetPath string, progressGroup *DownloadProgressGroup) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create download directory: %w", err)
	}

	response, err := d.svc.Files.Get(file.ID).
		SupportsAllDrives(true).
		Context(ctx).
		Download()
	if err != nil {
		return fmt.Errorf("download file %q: %w", file.Name, err)
	}
	defer response.Body.Close()

	output, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create file %q: %w", targetPath, err)
	}

	tracker := newProgressTracker("Downloading "+file.Name, file.Name, file.Size, progressGroup)
	if _, err := io.Copy(output, io.TeeReader(response.Body, tracker)); err != nil {
		_ = output.Close()
		return fmt.Errorf("write file %q: %w", targetPath, err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close file %q: %w", targetPath, err)
	}

	if file.Size > 0 {
		stat, err := os.Stat(targetPath)
		if err != nil {
			return fmt.Errorf("stat file %q: %w", targetPath, err)
		}
		if stat.Size() != file.Size {
			return fmt.Errorf("downloaded file %q has unexpected size: got %s, expected %s", file.Name, humanBytes(stat.Size()), humanBytes(file.Size))
		}
	}

	tracker.Finish()

	return nil
}

type progressTracker struct {
	label     string
	fileName  string
	total     int64
	current   int64
	lastLog   time.Time
	mu        sync.Mutex
	completed bool
	group     *DownloadProgressGroup
	line      *terminalLine
}

func newProgressTracker(label, fileName string, total int64, group *DownloadProgressGroup) *progressTracker {
	tracker := &progressTracker{
		label:    label,
		fileName: fileName,
		total:    total,
		group:    group,
	}
	if group == nil {
		tracker.line = newTerminalLine()
	}
	return tracker
}

func (p *progressTracker) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.current += int64(len(data))
	if p.group != nil {
		p.group.Update(p.fileName, p.current, p.total, false)
		return len(data), nil
	}

	if time.Since(p.lastLog) >= time.Second {
		p.logProgressLocked(false)
		p.lastLog = time.Now()
	}

	return len(data), nil
}

func (p *progressTracker) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.completed {
		return
	}
	p.completed = true
	if p.group != nil {
		p.group.Update(p.fileName, p.current, p.total, true)
		return
	}
	p.logProgressLocked(true)
}

func (p *progressTracker) logProgressLocked(done bool) {
	if p.total > 0 {
		percent := float64(p.current) / float64(p.total) * 100
		p.line.Render(fmt.Sprintf("%s: %.1f%% (%s/%s)", p.label, percent, humanBytes(p.current), humanBytes(p.total)), done)
		return
	}

	status := "in progress"
	if done {
		status = "done"
	}
	p.line.Render(fmt.Sprintf("%s: %s (%s)", p.label, status, humanBytes(p.current)), done)
}

func sameDay(left, right time.Time) bool {
	return left.Year() == right.Year() && left.Month() == right.Month() && left.Day() == right.Day()
}

func humanBytes(value int64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}

	divisor, exponent := int64(unit), 0
	for size := value / unit; size >= unit; size /= unit {
		divisor *= unit
		exponent++
	}

	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(divisor), "KMGTPE"[exponent])
}
