package internal

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/OpenNSW/nsw/oga/internal/database"
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
	Port            string
	DB              database.Config
	FormsPath       string
	DefaultFormID   string
	AllowedOrigins  []string
	NSW             NSWConfig
	MaxRequestBytes int64
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (Config, error) {
	driver := envOrDefault("OGA_DB_DRIVER", "sqlite")
	var dbConfig database.Config

	// Isolate required configurations per driver
	switch driver {
	case "postgres":
		password := os.Getenv("OGA_DB_PASSWORD")
		if password == "" {
			return Config{}, fmt.Errorf("database password secret is missing: OGA_DB_PASSWORD is required for postgres driver")
		}

		dbConfig = database.Config{
			Driver:   driver,
			Host:     envOrDefault("OGA_DB_HOST", "localhost"),
			Port:     envOrDefault("OGA_DB_PORT", "5432"),
			User:     envOrDefault("OGA_DB_USER", "postgres"),
			Password: password, // Uses the strictly validated password
			Name:     envOrDefault("OGA_DB_NAME", "oga_db"),
			SSLMode:  envOrDefault("OGA_DB_SSLMODE", "disable"),
		}

	case "sqlite":
		// SQLite only requires a file path
		dbConfig = database.Config{
			Driver: driver,
			Path:   envOrDefault("OGA_DB_PATH", "./oga_applications.db"),
		}

	default:
		return Config{}, fmt.Errorf("unsupported database driver configured: %s", driver)
	}

	cfg := Config{
		Port:           envOrDefault("OGA_PORT", "8081"),
		DB:             dbConfig,
		FormsPath:      envOrDefault("OGA_FORMS_PATH", "./data/forms"),
		DefaultFormID:  envOrDefault("OGA_DEFAULT_FORM_ID", "default"),
		AllowedOrigins: parseCommaSeparated(envOrDefault("OGA_ALLOWED_ORIGINS", "*")),
		NSW: NSWConfig{
			BaseURL:      os.Getenv("OGA_NSW_API_BASE_URL"),
			ClientID:     os.Getenv("OGA_NSW_CLIENT_ID"),
			ClientSecret: os.Getenv("OGA_NSW_CLIENT_SECRET"),
			TokenURL:     os.Getenv("OGA_NSW_TOKEN_URL"),
			Scopes:       parseCommaSeparated(os.Getenv("OGA_NSW_SCOPES")),
		},
	}

	maxRequestBytes, err := parseInt64Env("OGA_MAX_REQUEST_BYTES", 32<<20)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxRequestBytes = maxRequestBytes

	tokenInsecureSkipVerify, err := parseBoolEnv("OGA_NSW_TOKEN_INSECURE_SKIP_VERIFY", false)
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
		return fmt.Errorf("OGA_NSW_API_BASE_URL is required")
	}
	if strings.TrimSpace(c.NSW.ClientID) == "" {
		return fmt.Errorf("OGA_NSW_CLIENT_ID is required")
	}
	if strings.TrimSpace(c.NSW.ClientSecret) == "" {
		return fmt.Errorf("OGA_NSW_CLIENT_SECRET is required")
	}
	if strings.TrimSpace(c.NSW.TokenURL) == "" {
		return fmt.Errorf("OGA_NSW_TOKEN_URL is required")
	}
	return nil
}

func envOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
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
	raw := strings.TrimSpace(os.Getenv(key))
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
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue, nil
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid value for %s: %q", key, raw)
	}

	return value, nil
}
