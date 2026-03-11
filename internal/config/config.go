package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DateFormat = "2006-01-02"

type Config struct {
	DriveFolderID   string `yaml:"drive_folder_id"`
	DownloadDir     string `yaml:"download_dir"`
	CredentialsFile string `yaml:"credentials_file"`
	TokenFile       string `yaml:"token_file"`
}

func Load(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file %q: %w", path, err)
	}

	baseDir := filepath.Dir(path)
	if cfg.DownloadDir == "" {
		cfg.DownloadDir = "./tmp"
	}
	if cfg.CredentialsFile == "" {
		cfg.CredentialsFile = "./credentials.json"
	}
	if cfg.TokenFile == "" {
		cfg.TokenFile = "./token.json"
	}

	cfg.DownloadDir = resolvePath(baseDir, cfg.DownloadDir)
	cfg.CredentialsFile = resolvePath(baseDir, cfg.CredentialsFile)
	cfg.TokenFile = resolvePath(baseDir, cfg.TokenFile)

	if err := os.MkdirAll(cfg.DownloadDir, 0o755); err != nil {
		return nil, fmt.Errorf("create download directory %q: %w", cfg.DownloadDir, err)
	}

	return &cfg, nil
}

func (c *Config) ValidateForDrive() error {
	if strings.TrimSpace(c.DriveFolderID) == "" {
		return fmt.Errorf("drive_folder_id is required in config.yaml")
	}
	if strings.TrimSpace(c.CredentialsFile) == "" {
		return fmt.Errorf("credentials_file is required")
	}
	if strings.TrimSpace(c.TokenFile) == "" {
		return fmt.Errorf("token_file is required")
	}
	return nil
}

func (c *Config) ValidateForUpload() error {
	return c.ValidateForDrive()
}

func resolvePath(baseDir, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Clean(filepath.Join(baseDir, value))
}
