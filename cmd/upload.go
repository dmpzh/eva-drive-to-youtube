package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"eva-cli/internal/config"
	googleclient "eva-cli/internal/google"
	"eva-cli/internal/video"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"
)

var uploadDate string
var uploadSelection string

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Download selected sessions and upload them to YouTube as an unlisted video",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		selectedDate, err := resolveDateArg(uploadDate)
		if err != nil {
			return err
		}

		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		if err := cfg.ValidateForUpload(); err != nil {
			return err
		}

		authenticator := googleclient.NewAuthenticator(cfg.CredentialsFile, cfg.TokenFile)
		httpClient, err := authenticator.NewHTTPClient(ctx)
		if err != nil {
			return err
		}

		driveService, err := googleclient.NewDriveService(ctx, httpClient)
		if err != nil {
			return err
		}

		files, err := driveService.ListVideosByDate(ctx, cfg.DriveFolderID, selectedDate)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			fmt.Println("No sessions found.")
			return nil
		}

		printSessions(files)

		selectedIndexes, err := resolveSelection(len(files), uploadSelection)
		if err != nil {
			return err
		}

		selectedFiles := make([]googleclient.DriveFile, 0, len(selectedIndexes))
		for _, index := range selectedIndexes {
			selectedFiles = append(selectedFiles, files[index])
		}

		workingDir, err := os.MkdirTemp(cfg.DownloadDir, "eva-upload-*")
		if err != nil {
			return fmt.Errorf("create working directory: %w", err)
		}
		defer os.RemoveAll(workingDir)

		downloadedFiles, err := downloadFiles(ctx, driveService, workingDir, selectedFiles)
		if err != nil {
			return err
		}

		videoPath := downloadedFiles[0]
		if len(downloadedFiles) > 1 {
			ffmpeg := video.NewFFmpeg("ffmpeg")
			if err := ffmpeg.CheckAvailable(ctx); err != nil {
				return err
			}

			mergedPath := filepath.Join(workingDir, buildMergedFilename(selectedDate, selectedFiles))
			if err := video.MergeVideos(ctx, ffmpeg, downloadedFiles, mergedPath); err != nil {
				return err
			}
			videoPath = mergedPath
		}

		youtubeService, err := googleclient.NewYouTubeService(ctx, httpClient)
		if err != nil {
			return err
		}

		result, err := youtubeService.UploadVideo(ctx, videoPath, googleclient.UploadOptions{
			Title:         fmt.Sprintf("EVA VR - %s - Session Review", selectedDate.Format(config.DateFormat)),
			Description:   "EVA VR gameplay replay for analysis.",
			PrivacyStatus: "unlisted",
		})
		if err != nil {
			return err
		}

		fmt.Printf("Upload completed. Video ID: %s\n", result.Id)
		fmt.Printf("Watch URL: https://youtu.be/%s\n", result.Id)
		return nil
	},
}

func init() {
	uploadCmd.Flags().StringVar(&uploadDate, "date", "", "Date to upload (YYYY-MM-DD, today, or yesterday)")
	uploadCmd.Flags().StringVar(&uploadSelection, "sessions", "", "Session numbers to upload, for example: \"1 2\"")
	_ = uploadCmd.MarkFlagRequired("date")
	rootCmd.AddCommand(uploadCmd)
}

func resolveSelection(max int, flagValue string) ([]int, error) {
	input := strings.TrimSpace(flagValue)
	if input != "" {
		return parseSelection(input, max)
	}

	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
		selected, err := interactiveSelection(max)
		if err == nil {
			return selected, nil
		}
		fmt.Printf("Interactive selection unavailable, falling back to manual input: %v\n", err)
	}

	return manualSelection(max)
}

func interactiveSelection(max int) ([]int, error) {
	options := make([]string, max)
	for index := 0; index < max; index++ {
		options[index] = strconv.Itoa(index + 1)
	}

	selectedOptions := make([]string, 0, max)
	prompt := &survey.MultiSelect{
		Message:  "Select sessions to upload:",
		Options:  options,
		PageSize: max,
	}

	if err := survey.AskOne(prompt, &selectedOptions); err != nil {
		return nil, fmt.Errorf("interactive selection failed: %w", err)
	}

	return parseSelection(strings.Join(selectedOptions, " "), max)
}

func manualSelection(max int) ([]int, error) {
	fmt.Println()
	fmt.Println("Select sessions to upload:")
	fmt.Print("> ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read selection: %w", err)
	}

	return parseSelection(line, max)
}

func parseSelection(input string, max int) ([]int, error) {
	fields := strings.Fields(strings.ReplaceAll(input, ",", " "))
	if len(fields) == 0 {
		return nil, fmt.Errorf("no sessions selected")
	}

	seen := make(map[int]struct{}, len(fields))
	indexes := make([]int, 0, len(fields))
	for _, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil {
			return nil, fmt.Errorf("invalid selection %q", field)
		}
		if value < 1 || value > max {
			return nil, fmt.Errorf("selection %d out of range", value)
		}

		index := value - 1
		if _, exists := seen[index]; exists {
			continue
		}

		seen[index] = struct{}{}
		indexes = append(indexes, index)
	}

	sort.Ints(indexes)
	return indexes, nil
}

func downloadFiles(ctx context.Context, driveService *googleclient.DriveService, workingDir string, files []googleclient.DriveFile) ([]string, error) {
	paths := make([]string, len(files))
	group, groupCtx := errgroup.WithContext(ctx)

	for index, file := range files {
		index := index
		file := file
		group.Go(func() error {
			targetPath := filepath.Join(workingDir, fmt.Sprintf("%02d-%s", index+1, sanitizeFilename(file.Name)))
			if err := driveService.DownloadFile(groupCtx, file, targetPath); err != nil {
				return err
			}
			paths[index] = targetPath
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}

	return paths, nil
}

func buildMergedFilename(dateValue interface{ Format(string) string }, files []googleclient.DriveFile) string {
	times := make([]string, 0, len(files))
	for _, file := range files {
		times = append(times, file.CreatedAt.Format("15h04"))
	}

	return fmt.Sprintf("EVA VR - %s - Sessions %s.mp4", dateValue.Format(config.DateFormat), strings.Join(times, " + "))
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"<", "_",
		">", "_",
		":", "_",
		"\"", "_",
		"/", "_",
		"\\", "_",
		"|", "_",
		"?", "_",
		"*", "_",
	)

	cleaned := replacer.Replace(strings.TrimSpace(name))
	if cleaned == "" {
		return "video.mp4"
	}

	return cleaned
}
