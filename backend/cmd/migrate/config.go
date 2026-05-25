package main

import (
	"fmt"
	"os"

	"github.com/OpenNSW/nsw-agency/backend/internal/database"
)

// Config holds all configuration for the migrate command.
type Config struct {
	DB  database.Config
	Dir string // path to the SQL migration files directory
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (Config, error) {
	driver := envOrDefault("DB_DRIVER", "sqlite")
	var dbConfig database.Config

	switch driver {
	case "postgres":
		password := os.Getenv("DB_PASSWORD")
		if password == "" {
			return Config{}, fmt.Errorf("DB_PASSWORD is required for postgres driver")
		}
		dbConfig = database.Config{
			Driver:   driver,
			Host:     envOrDefault("DB_HOST", "localhost"),
			Port:     envOrDefault("DB_PORT", "5432"),
			User:     envOrDefault("DB_USER", "postgres"),
			Password: password,
			Name:     envOrDefault("DB_NAME", "nsw_agency_db"),
			SSLMode:  envOrDefault("DB_SSLMODE", "disable"),
		}

	case "sqlite":
		dbConfig = database.Config{
			Driver: driver,
			Path:   envOrDefault("DB_PATH", "./agency_applications.db"),
		}

	default:
		return Config{}, fmt.Errorf("unsupported database driver: %q", driver)
	}

	return Config{
		DB:  dbConfig,
		Dir: envOrDefault("MIGRATION_DIR", "./migrations"),
	}, nil
}

func envOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
