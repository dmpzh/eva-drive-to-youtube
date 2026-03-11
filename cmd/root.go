package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"eva-cli/internal/config"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "eva-cli",
	Short: "Upload EVA VR replay videos from Google Drive to YouTube",
	Long:  "eva-cli lists EVA VR replay videos from Google Drive, downloads selected sessions, optionally merges them, and uploads the result to YouTube.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.yaml", "Path to the configuration file")
}

func loadConfig() (*config.Config, error) {
	return config.Load(cfgFile)
}

func resolveDateArg(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("date is required")
	}

	now := time.Now()
	switch trimmed {
	case "today":
		return midnight(now), nil
	case "yesterday":
		return midnight(now.AddDate(0, 0, -1)), nil
	default:
		parsed, err := time.ParseInLocation(time.DateOnly, trimmed, now.Location())
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid date %q: expected YYYY-MM-DD, today, or yesterday", value)
		}
		return midnight(parsed), nil
	}
}

func midnight(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}
