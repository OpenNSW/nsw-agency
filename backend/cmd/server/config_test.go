package main

import (
	"strings"
	"testing"
	"time"
)

func setBaseConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_PATH", "./test.db")
}

func setRequiredNSWOAuth2Env(t *testing.T) {
	t.Helper()
	t.Setenv("NSW_API_BASE_URL", "http://localhost:8080/api/v1")
	t.Setenv("NSW_CLIENT_ID", "NPQS_TO_NSW")
	t.Setenv("NSW_CLIENT_SECRET", "secret")
	t.Setenv("NSW_TOKEN_URL", "https://localhost:8090/oauth2/token")
}

func TestLoadConfig_RequiresNSWOAuth2Vars(t *testing.T) {
	setBaseConfigEnv(t)
	setRequiredNSWOAuth2Env(t)

	testCases := []struct {
		name     string
		missing  string
		expected string
	}{
		{name: "missing api base url", missing: "NSW_API_BASE_URL", expected: "NSW_API_BASE_URL is required"},
		{name: "missing client id", missing: "NSW_CLIENT_ID", expected: "NSW_CLIENT_ID is required"},
		{name: "missing client secret", missing: "NSW_CLIENT_SECRET", expected: "NSW_CLIENT_SECRET is required"},
		{name: "missing token url", missing: "NSW_TOKEN_URL", expected: "NSW_TOKEN_URL is required"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.missing, "")
			_, err := LoadConfig()
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if err.Error() != tc.expected {
				t.Fatalf("expected error %q, got %q", tc.expected, err.Error())
			}
		})
	}
}

func TestLoadConfig_ParsesOptionalScopes(t *testing.T) {
	setBaseConfigEnv(t)
	setRequiredNSWOAuth2Env(t)
	t.Setenv("NSW_SCOPES", "scope.a, scope.b, ,scope.c")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := []string{"scope.a", "scope.b", "scope.c"}
	if len(cfg.NSW.Scopes) != len(expected) {
		t.Fatalf("expected %d scopes, got %d", len(expected), len(cfg.NSW.Scopes))
	}
	for i := range expected {
		if cfg.NSW.Scopes[i] != expected[i] {
			t.Fatalf("expected scope[%d]=%q, got %q", i, expected[i], cfg.NSW.Scopes[i])
		}
	}
}

func TestLoadConfig_AllowsEmptyScopes(t *testing.T) {
	setBaseConfigEnv(t)
	setRequiredNSWOAuth2Env(t)
	t.Setenv("NSW_SCOPES", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.NSW.Scopes) != 0 {
		t.Fatalf("expected empty scopes, got %v", cfg.NSW.Scopes)
	}
}

func TestLoadConfig_ParsesTokenInsecureSkipVerify(t *testing.T) {
	setBaseConfigEnv(t)
	setRequiredNSWOAuth2Env(t)
	t.Setenv("NSW_TOKEN_INSECURE_SKIP_VERIFY", "true")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.NSW.TokenInsecureSkipVerify {
		t.Fatalf("expected TokenInsecureSkipVerify to be true")
	}
}

func TestLoadConfig_RejectsInvalidTokenInsecureSkipVerify(t *testing.T) {
	setBaseConfigEnv(t)
	setRequiredNSWOAuth2Env(t)
	t.Setenv("NSW_TOKEN_INSECURE_SKIP_VERIFY", "not-a-bool")

	_, err := LoadConfig()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "invalid value for NSW_TOKEN_INSECURE_SKIP_VERIFY: \"not-a-bool\"" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_DefaultBlobSourcesAreDisabled(t *testing.T) {
	setBaseConfigEnv(t)
	setRequiredNSWOAuth2Env(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.FormsSource != nil {
		t.Errorf("expected FormsSource nil (disabled) by default, got %+v", cfg.FormsSource)
	}
	if cfg.TaskConfigsSource != nil {
		t.Errorf("expected TaskConfigsSource nil (disabled) by default, got %+v", cfg.TaskConfigsSource)
	}
}

func TestLoadConfig_ExplicitNoneIsDisabled(t *testing.T) {
	setBaseConfigEnv(t)
	setRequiredNSWOAuth2Env(t)
	t.Setenv("FORMS_SOURCE_TYPE", "none")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.FormsSource != nil {
		t.Errorf("expected FormsSource nil for TYPE=none, got %+v", cfg.FormsSource)
	}
}

func TestLoadConfig_FormsSourceLocal(t *testing.T) {
	setBaseConfigEnv(t)
	setRequiredNSWOAuth2Env(t)
	t.Setenv("FORMS_SOURCE_TYPE", "local")
	t.Setenv("FORMS_SOURCE_LOCAL_DIR", "/tmp/forms")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.FormsSource.Type != "local" || cfg.FormsSource.LocalDir != "/tmp/forms" {
		t.Errorf("unexpected FormsSource: %+v", cfg.FormsSource)
	}
}

func TestLoadConfig_FormsSourceLocalRequiresDir(t *testing.T) {
	setBaseConfigEnv(t)
	setRequiredNSWOAuth2Env(t)
	t.Setenv("FORMS_SOURCE_TYPE", "local")
	// FORMS_SOURCE_LOCAL_DIR intentionally unset.

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when FORMS_SOURCE_LOCAL_DIR is missing")
	}
	if !strings.Contains(err.Error(), "BLOBSOURCE_LOCAL_DIR") {
		t.Errorf("expected error to mention BLOBSOURCE_LOCAL_DIR, got %v", err)
	}
}

func TestLoadConfig_TaskConfigsSourceGitHub(t *testing.T) {
	setBaseConfigEnv(t)
	setRequiredNSWOAuth2Env(t)
	t.Setenv("TASK_CONFIGS_SOURCE_TYPE", "github")
	t.Setenv("TASK_CONFIGS_SOURCE_GITHUB_REPO", "OpenNSW/one-trade-agency-configs")
	t.Setenv("TASK_CONFIGS_SOURCE_GITHUB_REF", "main")
	t.Setenv("TASK_CONFIGS_SOURCE_GITHUB_REFRESH_INTERVAL", "5m")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.TaskConfigsSource.Type != "github" {
		t.Errorf("expected Type=github, got %q", cfg.TaskConfigsSource.Type)
	}
	if cfg.TaskConfigsSource.GitHubRepo != "OpenNSW/one-trade-agency-configs" {
		t.Errorf("unexpected Repo: %q", cfg.TaskConfigsSource.GitHubRepo)
	}
	if cfg.TaskConfigsSource.GitHubRefreshInterval != 5*time.Minute {
		t.Errorf("expected 5m refresh, got %v", cfg.TaskConfigsSource.GitHubRefreshInterval)
	}
}

func TestLoadConfig_GitHubSourceMissingRepoErrors(t *testing.T) {
	setBaseConfigEnv(t)
	setRequiredNSWOAuth2Env(t)
	t.Setenv("FORMS_SOURCE_TYPE", "github")
	t.Setenv("FORMS_SOURCE_GITHUB_REF", "main")
	// FORMS_SOURCE_GITHUB_REPO missing.

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when GITHUB_REPO is missing")
	}
	if !strings.Contains(err.Error(), "BLOBSOURCE_GITHUB_REPO") {
		t.Errorf("expected error to mention BLOBSOURCE_GITHUB_REPO, got %v", err)
	}
}

func TestLoadConfig_InvalidRefreshIntervalErrors(t *testing.T) {
	setBaseConfigEnv(t)
	setRequiredNSWOAuth2Env(t)
	// The refresh interval is only parsed when the source is enabled.
	t.Setenv("FORMS_SOURCE_TYPE", "github")
	t.Setenv("FORMS_SOURCE_GITHUB_REPO", "OpenNSW/one-trade-templates")
	t.Setenv("FORMS_SOURCE_GITHUB_REF", "main")
	t.Setenv("FORMS_SOURCE_GITHUB_REFRESH_INTERVAL", "not-a-duration")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid refresh interval")
	}
}
