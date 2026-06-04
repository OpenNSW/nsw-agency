package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/OpenNSW/nsw-agency/backend/internal/auth"
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

// FrontendConfig holds VITE_* values forwarded to the browser via /runtime-env.js.
// JSON field names must match exactly what window.__APP_CONFIG__ keys the frontend reads.
type FrontendConfig struct {
	InstanceConfig string `json:"VITE_INSTANCE_CONFIG"`
	BrandingName   string `json:"VITE_BRANDING_NAME"`
	APIBaseURL     string `json:"VITE_API_BASE_URL"`
	IDPBaseURL     string `json:"VITE_IDP_BASE_URL"`
	IDPClientID    string `json:"VITE_IDP_CLIENT_ID"`
	AppURL         string `json:"VITE_APP_URL"`
	IDPScopes      string `json:"VITE_IDP_SCOPES"`
	IDPPlatform    string `json:"VITE_IDP_PLATFORM"`
	IDPExpectedOU  string `json:"VITE_IDP_EXPECTED_OU_HANDLE"`
}

type Config struct {
	Port             string
	DB               database.Config
	TaskConfigsDir   string
	FormTemplatesDir string
	AllowedOrigins   []string
	NSW              NSWConfig
	Auth             auth.Config
	MaxRequestBytes  int64
	Frontend         FrontendConfig
	UIDir            string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (Config, error) {
	driver := envOrDefault("DB_DRIVER", "sqlite")
	var dbConfig database.Config

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
		dbConfig = database.Config{
			Driver: driver,
			Path:   envOrDefault("DB_PATH", "./agency_applications.db"),
		}

	default:
		return Config{}, fmt.Errorf("unsupported database driver configured: %s", driver)
	}

	taskConfigsDir := envOrDefault("TASK_CONFIGS_DIR", "./data/task-configs")
	formTemplatesDir := envOrDefault("FORM_TEMPLATES_DIR", "./data/forms")

	cfg := Config{
		Port:             envOrDefault("PORT", "8081"),
		DB:               dbConfig,
		TaskConfigsDir:   taskConfigsDir,
		FormTemplatesDir: formTemplatesDir,
		AllowedOrigins:   parseCommaSeparated(envOrDefault("ALLOWED_ORIGINS", "*")),
		NSW: NSWConfig{
			BaseURL:      os.Getenv("NSW_API_BASE_URL"),
			ClientID:     os.Getenv("NSW_CLIENT_ID"),
			ClientSecret: os.Getenv("NSW_CLIENT_SECRET"),
			TokenURL:     os.Getenv("NSW_TOKEN_URL"),
			Scopes:       parseCommaSeparated(os.Getenv("NSW_SCOPES")),
		},
		Auth: auth.Config{
			JWKSURL:    os.Getenv("AUTH_JWKS_URL"),
			Issuer:     os.Getenv("AUTH_ISSUER"),
			Audience:   os.Getenv("AUTH_AUDIENCE"),
			ClientIDs:  parseCommaSeparated(os.Getenv("AUTH_CLIENT_IDS")),
			ExpectedOU: os.Getenv("AUTH_EXPECTED_OU"),
		},
		Frontend: FrontendConfig{
			InstanceConfig: envOrDefault("VITE_INSTANCE_CONFIG", "npqs"),
			BrandingName:   envOrDefault("VITE_BRANDING_NAME", "default"),
			APIBaseURL:     os.Getenv("VITE_API_BASE_URL"), // empty = same-origin relative URLs
			IDPBaseURL:     os.Getenv("VITE_IDP_BASE_URL"),
			IDPClientID:    os.Getenv("VITE_IDP_CLIENT_ID"),
			AppURL:         os.Getenv("VITE_APP_URL"),
			IDPScopes:      envOrDefault("VITE_IDP_SCOPES", "openid,profile,email"),
			IDPPlatform:    envOrDefault("VITE_IDP_PLATFORM", "AsgardeoV2"),
			IDPExpectedOU:  os.Getenv("VITE_IDP_EXPECTED_OU_HANDLE"),
		},
		UIDir: envOrDefault("UI_DIR", "./ui"),
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

	authInsecureSkipTLSVerify, err := parseBoolEnv("AUTH_JWKS_INSECURE_SKIP_VERIFY", false)
	if err != nil {
		return Config{}, err
	}
	cfg.Auth.InsecureSkipTLSVerify = authInsecureSkipTLSVerify

	if err := cfg.validateNSWOAuth2Config(); err != nil {
		return Config{}, err
	}
	if err := cfg.Auth.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// validateFrontendConfig checks that the browser-required IDP variables are set
// when the server will serve the embedded frontend. Call this only after confirming
// UIDir exists on disk — there is no point failing when running in headless/API mode.
func (c Config) validateFrontendConfig() error {
	if strings.TrimSpace(c.Frontend.IDPBaseURL) == "" {
		return fmt.Errorf("VITE_IDP_BASE_URL is required when serving the frontend (UI_DIR=%s)", c.UIDir)
	}
	if strings.TrimSpace(c.Frontend.IDPClientID) == "" {
		return fmt.Errorf("VITE_IDP_CLIENT_ID is required when serving the frontend (UI_DIR=%s)", c.UIDir)
	}
	if strings.TrimSpace(c.Frontend.IDPExpectedOU) == "" {
		return fmt.Errorf("VITE_IDP_EXPECTED_OU_HANDLE is required when serving the frontend (UI_DIR=%s)", c.UIDir)
	}
	return nil
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
