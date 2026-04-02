package internal

import (
	"fmt"
	"os"
	"strings"

	"github.com/OpenNSW/nsw/oga/internal/database"
)

type NSWConfig struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	TokenURL     string
	Scopes       []string
}

type Config struct {
	Port           string
	DB             database.Config
	FormsPath      string
	DefaultFormID  string
	AllowedOrigins []string
	NSW            NSWConfig
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
		AllowedOrigins: parseOrigins(envOrDefault("OGA_ALLOWED_ORIGINS", "*")),
		NSW: NSWConfig{
			BaseURL:      envOrDefault("NSW_API_BASE_URL", "http://localhost:8080/api/v1"),
			ClientID:     os.Getenv("NSW_CLIENT_ID"),
			ClientSecret: os.Getenv("NSW_CLIENT_SECRET"),
			TokenURL:     os.Getenv("NSW_TOKEN_URL"),
		},
	}

	return cfg, nil
}

func envOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseOrigins(origins string) []string {
	if origins == "" {
		return []string{}
	}
	parts := strings.Split(origins, ",")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	return parts
}
