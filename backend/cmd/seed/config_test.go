package main

import "testing"

func TestSeedLoadConfig_Defaults(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.DB.Driver != "sqlite" {
		t.Errorf("DB.Driver = %q, want sqlite", cfg.DB.Driver)
	}
	if cfg.DB.Path != "./agency_applications.db" {
		t.Errorf("DB.Path = %q, want ./agency_applications.db", cfg.DB.Path)
	}
}

func TestSeedLoadConfig_SQLite(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_PATH", "./custom.db")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.DB.Driver != "sqlite" {
		t.Errorf("DB.Driver = %q, want sqlite", cfg.DB.Driver)
	}
	if cfg.DB.Path != "./custom.db" {
		t.Errorf("DB.Path = %q, want ./custom.db", cfg.DB.Path)
	}
}

func TestSeedLoadConfig_Postgres(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "admin")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "mydb")
	t.Setenv("DB_SSLMODE", "require")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.DB.Driver != "postgres" {
		t.Errorf("DB.Driver = %q, want postgres", cfg.DB.Driver)
	}
	if cfg.DB.Host != "db.example.com" {
		t.Errorf("DB.Host = %q, want db.example.com", cfg.DB.Host)
	}
	if cfg.DB.Port != "5433" {
		t.Errorf("DB.Port = %q, want 5433", cfg.DB.Port)
	}
	if cfg.DB.User != "admin" {
		t.Errorf("DB.User = %q, want admin", cfg.DB.User)
	}
	if cfg.DB.Password != "secret" {
		t.Errorf("DB.Password = %q, want secret", cfg.DB.Password)
	}
	if cfg.DB.Name != "mydb" {
		t.Errorf("DB.Name = %q, want mydb", cfg.DB.Name)
	}
	if cfg.DB.SSLMode != "require" {
		t.Errorf("DB.SSLMode = %q, want require", cfg.DB.SSLMode)
	}
}

func TestSeedLoadConfig_Postgres_RequiresPassword(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DB_PASSWORD", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when DB_PASSWORD is missing, got nil")
	}
}

func TestSeedLoadConfig_UnsupportedDriver(t *testing.T) {
	t.Setenv("DB_DRIVER", "mysql")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for unsupported driver, got nil")
	}
}
