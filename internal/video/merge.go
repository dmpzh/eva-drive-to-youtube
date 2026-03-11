package video

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func MergeVideos(ctx context.Context, ffmpeg *FFmpeg, inputFiles []string, outputPath string) error {
	if len(inputFiles) < 2 {
		return fmt.Errorf("at least two input files are required to merge videos")
	}

	listFilePath := filepath.Join(filepath.Dir(outputPath), "concat-list.txt")
	if err := os.WriteFile(listFilePath, []byte(buildConcatList(inputFiles)), 0o644); err != nil {
		return fmt.Errorf("write ffmpeg concat list: %w", err)
	}
	defer os.Remove(listFilePath)

	return ffmpeg.Concat(ctx, listFilePath, outputPath)
}

func buildConcatList(inputFiles []string) string {
	lines := make([]string, 0, len(inputFiles))
	for _, file := range inputFiles {
		absolutePath, err := filepath.Abs(file)
		if err != nil {
			absolutePath = file
		}
		escaped := strings.ReplaceAll(filepath.ToSlash(absolutePath), "'", "'\\''")
		lines = append(lines, fmt.Sprintf("file '%s'", escaped))
	}
	return strings.Join(lines, "\n") + "\n"
}
