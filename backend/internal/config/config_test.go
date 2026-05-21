package config

import "testing"

func setBaseConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("NSW_AGENCY_DB_DRIVER", "sqlite")
	t.Setenv("NSW_AGENCY_DB_PATH", "./test.db")
}

func setRequiredNSWOAuth2Env(t *testing.T) {
	t.Helper()
	t.Setenv("NSW_AGENCY_NSW_API_BASE_URL", "http://localhost:8080/api/v1")
	t.Setenv("NSW_AGENCY_NSW_CLIENT_ID", "NPQS_TO_NSW")
	t.Setenv("NSW_AGENCY_NSW_CLIENT_SECRET", "secret")
	t.Setenv("NSW_AGENCY_NSW_TOKEN_URL", "https://localhost:8090/oauth2/token")
}

func TestLoadConfig_RequiresNSWOAuth2Vars(t *testing.T) {
	setBaseConfigEnv(t)
	setRequiredNSWOAuth2Env(t)

	testCases := []struct {
		name     string
		missing  string
		expected string
	}{
		{name: "missing api base url", missing: "NSW_AGENCY_NSW_API_BASE_URL", expected: "NSW_API_BASE_URL is required"},
		{name: "missing client id", missing: "NSW_AGENCY_NSW_CLIENT_ID", expected: "NSW_CLIENT_ID is required"},
		{name: "missing client secret", missing: "NSW_AGENCY_NSW_CLIENT_SECRET", expected: "NSW_CLIENT_SECRET is required"},
		{name: "missing token url", missing: "NSW_AGENCY_NSW_TOKEN_URL", expected: "NSW_TOKEN_URL is required"},
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
	t.Setenv("NSW_AGENCY_NSW_SCOPES", "scope.a, scope.b, ,scope.c")

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
	t.Setenv("NSW_AGENCY_NSW_SCOPES", "")

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
	t.Setenv("NSW_AGENCY_NSW_TOKEN_INSECURE_SKIP_VERIFY", "true")

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
	t.Setenv("NSW_AGENCY_NSW_TOKEN_INSECURE_SKIP_VERIFY", "not-a-bool")

	_, err := LoadConfig()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "invalid value for NSW_AGENCY_NSW_TOKEN_INSECURE_SKIP_VERIFY: \"not-a-bool\"" {
		t.Fatalf("unexpected error: %v", err)
	}
}
