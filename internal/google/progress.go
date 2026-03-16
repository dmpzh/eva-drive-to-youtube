package googleclient

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

var spinnerFrames = []string{"|", "/", "-", "\\"}

const interactiveRefreshInterval = 400 * time.Millisecond

type terminalLine struct {
	mu          sync.Mutex
	interactive bool
	lastWidth   int
	lastLog     time.Time
	frameIndex  int
}

func newTerminalLine() *terminalLine {
	return &terminalLine{interactive: term.IsTerminal(int(os.Stdout.Fd()))}
}

func (t *terminalLine) Render(text string, done bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.interactive {
		now := time.Now()
		if !done && !t.lastLog.IsZero() && now.Sub(t.lastLog) < interactiveRefreshInterval {
			return
		}

		frame := spinnerFrames[t.frameIndex%len(spinnerFrames)]
		t.frameIndex++

		line := text
		if !done {
			line = fmt.Sprintf("[%s] %s", frame, text)
		}

		padding := ""
		if t.lastWidth > len(line) {
			padding = strings.Repeat(" ", t.lastWidth-len(line))
		}

		fmt.Printf("\r%s%s", line, padding)
		t.lastWidth = len(line)
		t.lastLog = now
		if done {
			fmt.Print("\n")
			t.lastWidth = 0
		}
		return
	}

	if !done && time.Since(t.lastLog) < time.Second {
		return
	}

	fmt.Println(text)
	t.lastLog = time.Now()
}

type DownloadProgressGroup struct {
	mu             sync.Mutex
	line           *terminalLine
	totalFiles     int
	completedFiles int
	totalBytes     int64
	currentBytes   int64
	fileProgress   map[string]int64
	fileDone       map[string]bool
}

func NewDownloadProgressGroup(files []DriveFile) *DownloadProgressGroup {
	group := &DownloadProgressGroup{
		line:         newTerminalLine(),
		totalFiles:   len(files),
		fileProgress: make(map[string]int64, len(files)),
		fileDone:     make(map[string]bool, len(files)),
	}

	for _, file := range files {
		if file.Size > 0 {
			group.totalBytes += file.Size
		}
	}

	return group
}

func (g *DownloadProgressGroup) Update(fileName string, current, total int64, done bool) {
	g.mu.Lock()

	previous := g.fileProgress[fileName]
	if current < previous {
		current = previous
	}

	g.currentBytes += current - previous
	g.fileProgress[fileName] = current

	if done && !g.fileDone[fileName] {
		g.fileDone[fileName] = true
		g.completedFiles++
	}

	completedFiles := g.completedFiles
	totalFiles := g.totalFiles
	currentBytes := g.currentBytes
	totalBytes := g.totalBytes
	allDone := totalFiles > 0 && completedFiles == totalFiles

	g.mu.Unlock()

	text := formatDownloadSummary(completedFiles, totalFiles, currentBytes, totalBytes)
	g.line.Render(text, allDone)

	_ = total
	if totalFiles == 0 {
		g.line.Render("Downloads completed.", true)
	}
}

func formatDownloadSummary(completedFiles, totalFiles int, currentBytes, totalBytes int64) string {
	if totalBytes > 0 {
		percent := float64(currentBytes) / float64(totalBytes) * 100
		return fmt.Sprintf("Downloading files %d/%d | %.1f%% (%s/%s)", completedFiles, totalFiles, percent, humanBytes(currentBytes), humanBytes(totalBytes))
	}

	return fmt.Sprintf("Downloading files %d/%d | %s", completedFiles, totalFiles, humanBytes(currentBytes))
}
