package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds all application configuration loaded from config.json
type Config struct {
	Port            int      `json:"port"`
	DBPath          string   `json:"db_path"`
	Username        string   `json:"username"`
	Password        string   `json:"password"`
	JWTSecret       string   `json:"jwt_secret"`
	AnthropicAPIKey string   `json:"anthropic_api_key"`
	MCPServiceToken string   `json:"mcp_service_token"`
	AllowedOrigins  []string `json:"allowed_origins"`
}

// Load reads and parses config.json, applying sensible defaults.
func Load() (Config, error) {
	cfg := Config{
		Port:     8080,
		DBPath:   "recipe_manager.db",
		Username: "test",
		Password: "test",
	}
	f, err := os.Open("config.json")
	if err != nil {
		return cfg, fmt.Errorf("opening config.json: %w", err)
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("parsing config.json: %w", err)
	}
	return cfg, nil
}

// Validate returns an error if required fields are missing.
func (c Config) Validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("jwt_secret is required")
	}
	return nil
}
