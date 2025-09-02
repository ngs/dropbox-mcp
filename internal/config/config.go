package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".dropbox-mcp", "config.json"), nil
}

func Load() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Save() error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(configPath)
	if mkdirErr := os.MkdirAll(dir, 0o700); mkdirErr != nil {
		return fmt.Errorf("failed to create config directory: %w", mkdirErr)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *Config) IsTokenValid() bool {
	if c.AccessToken == "" {
		return false
	}
	if c.ExpiresAt.IsZero() {
		return true
	}
	return time.Now().Before(c.ExpiresAt)
}

func (c *Config) NeedsRefresh() bool {
	if c.RefreshToken == "" {
		return false
	}
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().Add(5 * time.Minute).After(c.ExpiresAt)
}

func (c *Config) UpdateTokens(accessToken, refreshToken string, expiresAt time.Time) {
	c.AccessToken = accessToken
	if refreshToken != "" {
		c.RefreshToken = refreshToken
	}
	c.ExpiresAt = expiresAt
}
