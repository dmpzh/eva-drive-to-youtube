package video

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type FFmpeg struct {
	Binary      string
	ProbeBinary string
}

func NewFFmpeg(binary string) *FFmpeg {
	if binary == "" {
		binary = "ffmpeg"
	}
	return &FFmpeg{
		Binary:      binary,
		ProbeBinary: deriveFFprobeBinary(binary),
	}
}

func (f *FFmpeg) CheckAvailable(ctx context.Context) error {
	if _, err := exec.LookPath(f.Binary); err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}
	if _, err := exec.LookPath(f.ProbeBinary); err != nil {
		return fmt.Errorf("ffprobe not found in PATH: %w", err)
	}
	return nil
}

func (f *FFmpeg) Concat(ctx context.Context, listFilePath, outputPath string) error {
	command := exec.CommandContext(ctx, f.Binary,
		"-y",
		"-xerror",
		"-f", "concat",
		"-safe", "0",
		"-i", listFilePath,
		"-c", "copy",
		outputPath,
	)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	if err := command.Run(); err != nil {
		return fmt.Errorf("run ffmpeg concat: %w", err)
	}

	return nil
}

func (f *FFmpeg) ProbeDuration(ctx context.Context, filePath string) (time.Duration, error) {
	command := exec.CommandContext(ctx, f.ProbeBinary,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath,
	)

	output, err := command.Output()
	if err != nil {
		return 0, fmt.Errorf("run ffprobe for %q: %w", filePath, err)
	}

	duration, err := parseProbeDuration(string(output))
	if err != nil {
		return 0, fmt.Errorf("parse ffprobe output for %q: %w", filePath, err)
	}

	return duration, nil
}

func deriveFFprobeBinary(ffmpegBinary string) string {
	base := filepath.Base(ffmpegBinary)
	if base == "ffmpeg" {
		return filepath.Join(filepath.Dir(ffmpegBinary), "ffprobe")
	}
	if base == "ffmpeg.exe" {
		return filepath.Join(filepath.Dir(ffmpegBinary), "ffprobe.exe")
	}
	return "ffprobe"
}
