package cmd

import (
	"context"

	googleclient "eva-cli/internal/google"

	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Perform OAuth2 login for Google Drive and YouTube",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		authenticator := googleclient.NewAuthenticator(cfg.CredentialsFile, cfg.TokenFile)
		return authenticator.Login(context.Background())
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
