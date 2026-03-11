package video

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

type FFmpeg struct {
	Binary string
}

func NewFFmpeg(binary string) *FFmpeg {
	if binary == "" {
		binary = "ffmpeg"
	}
	return &FFmpeg{Binary: binary}
}

func (f *FFmpeg) CheckAvailable(ctx context.Context) error {
	if _, err := exec.LookPath(f.Binary); err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}
	return nil
}

func (f *FFmpeg) Concat(ctx context.Context, listFilePath, outputPath string) error {
	command := exec.CommandContext(ctx, f.Binary,
		"-y",
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
