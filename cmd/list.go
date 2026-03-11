package cmd

import (
	"context"
	"fmt"

	googleclient "eva-cli/internal/google"

	"github.com/spf13/cobra"
)

var listDate string

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List replay sessions from Google Drive for a given date",
	RunE: func(cmd *cobra.Command, args []string) error {
		selectedDate, err := resolveDateArg(listDate)
		if err != nil {
			return err
		}

		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		if err := cfg.ValidateForDrive(); err != nil {
			return err
		}

		authenticator := googleclient.NewAuthenticator(cfg.CredentialsFile, cfg.TokenFile)
		httpClient, err := authenticator.NewHTTPClient(context.Background())
		if err != nil {
			return err
		}

		driveService, err := googleclient.NewDriveService(context.Background(), httpClient)
		if err != nil {
			return err
		}

		files, err := driveService.ListVideosByDate(context.Background(), cfg.DriveFolderID, selectedDate)
		if err != nil {
			return err
		}

		printSessions(files)
		return nil
	},
}

func init() {
	listCmd.Flags().StringVar(&listDate, "date", "", "Date to list (YYYY-MM-DD, today, or yesterday)")
	_ = listCmd.MarkFlagRequired("date")
	rootCmd.AddCommand(listCmd)
}

func printSessions(files []googleclient.DriveFile) {
	if len(files) == 0 {
		fmt.Println("No sessions found.")
		return
	}

	fmt.Println("Sessions found:")
	for index, file := range files {
		fmt.Printf("%d. %s\n", index+1, file.CreatedAt.Format("2006-01-02 15:04"))
	}
}
