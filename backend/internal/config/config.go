package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/OpenNSW/nsw-agency/backend/internal/database"
)

type NSWConfig struct {
	BaseURL                 string
	ClientID                string
	ClientSecret            string
	TokenURL                string
	Scopes                  []string
	TokenInsecureSkipVerify bool
}

type Config struct {
	Port                string
	DB                  database.Config
	ConfigDir           string
	DefaultTaskConfigID string
	AllowedOrigins      []string
	NSW                 NSWConfig
	MaxRequestBytes     int64
}

func getEnv(key string) string {
	if !strings.HasPrefix(key, "NSW_AGENCY_") {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return os.Getenv("NSW_AGENCY_" + key)
	}
	k := strings.TrimPrefix(key, "NSW_AGENCY_")
	if value := os.Getenv(k); value != "" {
		return value
	}
	return os.Getenv(key)
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (Config, error) {
	driver := envOrDefault("NSW_AGENCY_DB_DRIVER", "sqlite")
	var dbConfig database.Config

	// Isolate required configurations per driver
	switch driver {
	case "postgres":
		password := getEnv("NSW_AGENCY_DB_PASSWORD")
		if password == "" {
			return Config{}, fmt.Errorf("database password secret is missing: DB_PASSWORD is required for postgres driver")
		}

		dbConfig = database.Config{
			Driver:   driver,
			Host:     envOrDefault("NSW_AGENCY_DB_HOST", "localhost"),
			Port:     envOrDefault("NSW_AGENCY_DB_PORT", "5432"),
			User:     envOrDefault("NSW_AGENCY_DB_USER", "postgres"),
			Password: password, // Uses the strictly validated password
			Name:     envOrDefault("NSW_AGENCY_DB_NAME", "nsw_agency_db"),
			SSLMode:  envOrDefault("NSW_AGENCY_DB_SSLMODE", "disable"),
		}

	case "sqlite":
		// SQLite only requires a file path
		dbConfig = database.Config{
			Driver: driver,
			Path:   envOrDefault("NSW_AGENCY_DB_PATH", "./nsw_agency_applications.db"),
		}

	default:
		return Config{}, fmt.Errorf("unsupported database driver configured: %s", driver)
	}

	cfg := Config{
		Port:                envOrDefault("NSW_AGENCY_PORT", "8081"),
		DB:                  dbConfig,
		ConfigDir:           envOrDefault("NSW_AGENCY_CONFIG_DIR", "./data"),
		DefaultTaskConfigID: envOrDefault("NSW_AGENCY_DEFAULT_TASK_CONFIG_ID", "default"),
		AllowedOrigins:      parseCommaSeparated(envOrDefault("NSW_AGENCY_ALLOWED_ORIGINS", "*")),
		NSW: NSWConfig{
			BaseURL:      getEnv("NSW_AGENCY_NSW_API_BASE_URL"),
			ClientID:     getEnv("NSW_AGENCY_NSW_CLIENT_ID"),
			ClientSecret: getEnv("NSW_AGENCY_NSW_CLIENT_SECRET"),
			TokenURL:     getEnv("NSW_AGENCY_NSW_TOKEN_URL"),
			Scopes:       parseCommaSeparated(getEnv("NSW_AGENCY_NSW_SCOPES")),
		},
	}

	maxRequestBytes, err := parseInt64Env("NSW_AGENCY_MAX_REQUEST_BYTES", 32<<20)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxRequestBytes = maxRequestBytes

	tokenInsecureSkipVerify, err := parseBoolEnv("NSW_AGENCY_NSW_TOKEN_INSECURE_SKIP_VERIFY", false)
	if err != nil {
		return Config{}, err
	}
	cfg.NSW.TokenInsecureSkipVerify = tokenInsecureSkipVerify

	if err := cfg.validateNSWOAuth2Config(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) validateNSWOAuth2Config() error {
	if strings.TrimSpace(c.NSW.BaseURL) == "" {
		return fmt.Errorf("NSW_API_BASE_URL is required")
	}
	if strings.TrimSpace(c.NSW.ClientID) == "" {
		return fmt.Errorf("NSW_CLIENT_ID is required")
	}
	if strings.TrimSpace(c.NSW.ClientSecret) == "" {
		return fmt.Errorf("NSW_CLIENT_SECRET is required")
	}
	if strings.TrimSpace(c.NSW.TokenURL) == "" {
		return fmt.Errorf("NSW_TOKEN_URL is required")
	}
	return nil
}

func envOrDefault(key, defaultValue string) string {
	if value := getEnv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseCommaSeparated(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseBoolEnv(key string, defaultValue bool) (bool, error) {
	raw := strings.TrimSpace(getEnv(key))
	if raw == "" {
		return defaultValue, nil
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("invalid value for %s: %q", key, raw)
	}

	return value, nil
}

func parseInt64Env(key string, defaultValue int64) (int64, error) {
	raw := strings.TrimSpace(getEnv(key))
	if raw == "" {
		return defaultValue, nil
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid value for %s: %q", key, raw)
	}

	return value, nil
}
