package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/OpenNSW/nsw-agency/backend/internal/database"
	"github.com/OpenNSW/nsw-agency/backend/pkg/blobsource"
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
	// FormsSource / TaskConfigsSource are the optional primary blob sources.
	// nil means "no primary source" (disabled) — only the built-in defaults
	// under ConfigDir are used.
	FormsSource       *blobsource.Config
	TaskConfigsSource *blobsource.Config
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (Config, error) {
	driver := envOrDefault("DB_DRIVER", "sqlite")
	var dbConfig database.Config

	// Isolate required configurations per driver
	switch driver {
	case "postgres":
		password := os.Getenv("DB_PASSWORD")
		if password == "" {
			return Config{}, fmt.Errorf("database password secret is missing: DB_PASSWORD is required for postgres driver")
		}

		dbConfig = database.Config{
			Driver:   driver,
			Host:     envOrDefault("DB_HOST", "localhost"),
			Port:     envOrDefault("DB_PORT", "5432"),
			User:     envOrDefault("DB_USER", "postgres"),
			Password: password, // Uses the strictly validated password
			Name:     envOrDefault("DB_NAME", "nsw_agency_db"),
			SSLMode:  envOrDefault("DB_SSLMODE", "disable"),
		}

	case "sqlite":
		// SQLite only requires a file path
		dbConfig = database.Config{
			Driver: driver,
			Path:   envOrDefault("DB_PATH", "./agency_applications.db"),
		}

	default:
		return Config{}, fmt.Errorf("unsupported database driver configured: %s", driver)
	}

	formsSource, err := loadBlobSourceConfig("FORMS_SOURCE")
	if err != nil {
		return Config{}, err
	}
	taskConfigsSource, err := loadBlobSourceConfig("TASK_CONFIGS_SOURCE")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Port:                envOrDefault("PORT", "8081"),
		DB:                  dbConfig,
		ConfigDir:           envOrDefault("CONFIG_DIR", "./data"),
		DefaultTaskConfigID: envOrDefault("DEFAULT_TASK_CONFIG_ID", "default"),
		AllowedOrigins:      parseCommaSeparated(envOrDefault("ALLOWED_ORIGINS", "*")),
		NSW: NSWConfig{
			BaseURL:      os.Getenv("NSW_API_BASE_URL"),
			ClientID:     os.Getenv("NSW_CLIENT_ID"),
			ClientSecret: os.Getenv("NSW_CLIENT_SECRET"),
			TokenURL:     os.Getenv("NSW_TOKEN_URL"),
			Scopes:       parseCommaSeparated(os.Getenv("NSW_SCOPES")),
		},
		FormsSource:       formsSource,
		TaskConfigsSource: taskConfigsSource,
	}
	maxRequestBytes, err := parseInt64Env("MAX_REQUEST_BYTES", 32<<20)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxRequestBytes = maxRequestBytes

	tokenInsecureSkipVerify, err := parseBoolEnv("NSW_TOKEN_INSECURE_SKIP_VERIFY", false)
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

// loadBlobSourceConfig reads <PREFIX>_TYPE and friends from the environment
// and returns the corresponding blobsource.Config, or nil when the source is
// disabled (TYPE unset or "none"). A returned config is validated, so bad
// values fail at startup rather than on first request.
func loadBlobSourceConfig(prefix string) (*blobsource.Config, error) {
	typ := strings.TrimSpace(os.Getenv(prefix + "_TYPE"))
	if typ == "" || typ == "none" {
		return nil, nil
	}
	refreshInterval, err := parseDurationEnv(prefix+"_GITHUB_REFRESH_INTERVAL", 0)
	if err != nil {
		return nil, err
	}
	cfg := blobsource.Config{
		Type:                  typ,
		LocalDir:              os.Getenv(prefix + "_LOCAL_DIR"),
		GitHubRepo:            os.Getenv(prefix + "_GITHUB_REPO"),
		GitHubRef:             os.Getenv(prefix + "_GITHUB_REF"),
		GitHubManifestPath:    os.Getenv(prefix + "_GITHUB_MANIFEST_PATH"),
		GitHubBaseURL:         os.Getenv(prefix + "_GITHUB_BASE_URL"),
		GitHubRefreshInterval: refreshInterval,
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", prefix, err)
	}
	return &cfg, nil
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

func parseDurationEnv(key string, defaultValue time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid value for %s: %q", key, raw)
	}
	return value, nil
}
