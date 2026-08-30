package config

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the application configuration settings.
type Config struct {
	Port               string
	DatabaseURL        string
	SessionSecret      string
	Environment        string
	IsRender           bool
	MaxOpenConns       int
	MaxIdleConns       int
	ConnMaxLifetime    time.Duration
	ConnMaxIdleTime    time.Duration
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	CORSAllowedOrigins []string
	RateLimitRPS       float64
	RateLimitBurst     int
	KeepAliveEnabled   bool
	KeepAliveURL       string
	KeepAliveInterval  time.Duration
}

// Load loads configuration from environment variables, automatically loading .env if available.
func Load() *Config {
	// Auto-load .env file if present
	loadDotEnv(".env")

	port := getEnv("PORT", "8080")
	dbURL := resolveDatabaseURL()
	sessionSecret := getEnv("SESSION_SECRET", "default-dev-secret-replace-in-production-32b")

	isRender := os.Getenv("RENDER") == "true" || os.Getenv("RENDER_SERVICE_ID") != ""
	env := getEnv("ENVIRONMENT", "development")
	if isRender && env == "development" {
		env = "production"
	}

	// CORS origins
	corsOriginsStr := getEnv("CORS_ALLOWED_ORIGINS", "*")
	var corsOrigins []string
	for _, o := range strings.Split(corsOriginsStr, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			corsOrigins = append(corsOrigins, trimmed)
		}
	}
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"*"}
	}

	// Rate limiting settings (default 20 req/s, burst 60)
	rps, _ := strconv.ParseFloat(getEnv("RATE_LIMIT_RPS", "20"), 64)
	if rps <= 0 {
		rps = 20
	}
	burst, _ := strconv.Atoi(getEnv("RATE_LIMIT_BURST", "60"))
	if burst <= 0 {
		burst = 60
	}

	// Keep-alive anti-sleep settings (Render free-tier pinger)
	keepAliveURL := getEnv("KEEP_ALIVE_URL", "")
	if keepAliveURL == "" && isRender {
		// Default to Render service external URL if known
		if extURL := os.Getenv("RENDER_EXTERNAL_URL"); extURL != "" {
			keepAliveURL = extURL + "/healthz"
		} else {
			keepAliveURL = "https://job-tracker-server-9drb.onrender.com/healthz"
		}
	}

	keepAliveIntervalMin, _ := strconv.Atoi(getEnv("KEEP_ALIVE_INTERVAL_MINUTES", "10"))
	if keepAliveIntervalMin <= 0 {
		keepAliveIntervalMin = 10
	}

	keepAliveEnabled := isRender || keepAliveURL != ""
	if disabledStr := os.Getenv("KEEP_ALIVE_ENABLED"); disabledStr == "false" || disabledStr == "0" {
		keepAliveEnabled = false
	}

	return &Config{
		Port:               port,
		DatabaseURL:        dbURL,
		SessionSecret:      sessionSecret,
		Environment:        env,
		IsRender:           isRender,
		MaxOpenConns:       15,
		MaxIdleConns:       5,
		ConnMaxLifetime:    15 * time.Minute,
		ConnMaxIdleTime:    5 * time.Minute,
		ReadTimeout:        15 * time.Second,
		WriteTimeout:       20 * time.Second,
		IdleTimeout:        60 * time.Second,
		CORSAllowedOrigins: corsOrigins,
		RateLimitRPS:       rps,
		RateLimitBurst:     burst,
		KeepAliveEnabled:   keepAliveEnabled,
		KeepAliveURL:       keepAliveURL,
		KeepAliveInterval:  time.Duration(keepAliveIntervalMin) * time.Minute,
	}
}

// resolveDatabaseURL resolves the Postgres connection string from various cloud/Render formats.
func resolveDatabaseURL() string {
	// 1. Direct environment variables commonly used on Render & cloud hosts
	directKeys := []string{
		"DATABASE_URL",
		"INTERNAL_DATABASE_URL", // Render internal PostgreSQL URL
		"RENDER_DATABASE_URL",
		"POSTGRES_URL",
		"POSTGRESQL_URL",
	}

	for _, key := range directKeys {
		if val := strings.TrimSpace(os.Getenv(key)); val != "" {
			return normalizeDatabaseURL(val)
		}
	}

	// 2. Discrete database environment parameters
	dbHost := os.Getenv("DB_HOST")
	if dbHost != "" {
		dbPort := getEnv("DB_PORT", "5432")
		dbUser := getEnv("DB_USER", "postgres")
		dbPass := os.Getenv("DB_PASSWORD")
		dbName := getEnv("DB_NAME", "jobtracker_db")
		sslMode := getEnv("DB_SSLMODE", "require")

		userPass := url.User(dbUser)
		if dbPass != "" {
			userPass = url.UserPassword(dbUser, dbPass)
		}

		u := &url.URL{
			Scheme: "postgres",
			User:   userPass,
			Host:   fmt.Sprintf("%s:%s", dbHost, dbPort),
			Path:   "/" + dbName,
		}
		q := u.Query()
		q.Set("sslmode", sslMode)
		u.RawQuery = q.Encode()

		return u.String()
	}

	// 3. Fallback default for local docker-compose development
	return "postgres://jobtracker_user:jobtracker_pass@localhost:5432/jobtracker_db?sslmode=disable"
}

// normalizeDatabaseURL handles url compatibility adjustments if needed
func normalizeDatabaseURL(dbURL string) string {
	return dbURL
}

// loadDotEnv reads key-value pairs from a .env file if it exists without external libraries.
func loadDotEnv(filenames ...string) {
	for _, filename := range filenames {
		file, err := os.Open(filename)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])

			// Strip quotes if present
			if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
				val = val[1 : len(val)-1]
			}

			// Only set if not already set in environment
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, val)
			}
		}
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
