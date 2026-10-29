package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port          string
	DatabaseURL   string
	JWTSecret     string
	JWTIssuer     string
	JWTAudience   string
	ServiceAPIKey string
	LogLevel      string
}

// Load reads typed configuration from environment variables only.
func Load() (*Config, error) {
	cfg := &Config{
		Port:          getEnv("PORT", "8092"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		JWTIssuer:     getEnv("JWT_ISSUER", "goodfood-auth"),
		JWTAudience:   getEnv("JWT_AUDIENCE", "goodfood-api"),
		ServiceAPIKey: os.Getenv("SERVICE_API_KEY"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.ServiceAPIKey == "" {
		return nil, fmt.Errorf("SERVICE_API_KEY is required")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
