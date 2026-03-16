package video

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func validateVideoSet(ctx context.Context, ffmpeg *FFmpeg, inputFiles []string) (time.Duration, error) {
	var totalDuration time.Duration

	for _, inputFile := range inputFiles {
		duration, err := ffmpeg.ProbeDuration(ctx, inputFile)
		if err != nil {
			return 0, fmt.Errorf("validate input video %q: %w", filepath.Base(inputFile), err)
		}
		totalDuration += duration
	}

	return totalDuration, nil
}

func ensureMergedDuration(ctx context.Context, ffmpeg *FFmpeg, outputPath string, expected time.Duration, inputCount int) error {
	mergedDuration, err := ffmpeg.ProbeDuration(ctx, outputPath)
	if err != nil {
		return fmt.Errorf("validate merged video %q: %w", filepath.Base(outputPath), err)
	}

	tolerance := time.Duration(maxInt(3, inputCount*2)) * time.Second
	if mergedDuration+tolerance < expected {
		return fmt.Errorf("merged video is shorter than expected: got %s, expected about %s", formatDuration(mergedDuration), formatDuration(expected))
	}

	return nil
}

func parseProbeDuration(raw string) (time.Duration, error) {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, err
	}

	return time.Duration(math.Round(seconds * float64(time.Second))), nil
}

func formatDuration(value time.Duration) string {
	if value < 0 {
		value = 0
	}

	totalSeconds := int(value.Round(time.Second) / time.Second)
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
