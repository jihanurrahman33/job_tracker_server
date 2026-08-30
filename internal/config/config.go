package config

import (
	"os"
	"time"
)

// Config holds the application configuration settings.
type Config struct {
	Port          string
	DatabaseURL   string
	SessionSecret string
	Environment   string
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	IdleTimeout   time.Duration
}

// Load loads configuration from environment variables with sensible defaults.
func Load() *Config {
	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://jobtracker_user:jobtracker_pass@localhost:5432/jobtracker_db?sslmode=disable")
	sessionSecret := getEnv("SESSION_SECRET", "default-dev-secret-replace-in-production-32b")
	env := getEnv("ENVIRONMENT", "development")

	return &Config{
		Port:          port,
		DatabaseURL:   dbURL,
		SessionSecret: sessionSecret,
		Environment:   env,
		ReadTimeout:   10 * time.Second,
		WriteTimeout:  15 * time.Second,
		IdleTimeout:   60 * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
