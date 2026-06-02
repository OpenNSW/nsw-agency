package main

import "testing"

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

func setRequiredAuthEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AUTH_JWKS_URL", "https://localhost:8090/.well-known/jwks.json")
	t.Setenv("AUTH_ISSUER", "https://localhost:8090")
	t.Setenv("AUTH_AUDIENCE", "OGA_PORTAL_APP")
	t.Setenv("AUTH_CLIENT_IDS", "OGA_PORTAL_APP")
	t.Setenv("AUTH_EXPECTED_OU", "default")
}

func TestLoadConfig_RequiresNSWOAuth2Vars(t *testing.T) {
	setBaseConfigEnv(t)
	setRequiredNSWOAuth2Env(t)
	setRequiredAuthEnv(t)

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

func TestLoadConfig_RequiresAuthVars(t *testing.T) {
	setBaseConfigEnv(t)
	setRequiredNSWOAuth2Env(t)
	setRequiredAuthEnv(t)

	testCases := []struct {
		name     string
		missing  string
		expected string
	}{
		{name: "missing jwks url", missing: "AUTH_JWKS_URL", expected: "AUTH_JWKS_URL is required"},
		{name: "missing issuer", missing: "AUTH_ISSUER", expected: "AUTH_ISSUER is required"},
		{name: "missing audience", missing: "AUTH_AUDIENCE", expected: "AUTH_AUDIENCE is required"},
		{name: "missing client ids", missing: "AUTH_CLIENT_IDS", expected: "AUTH_CLIENT_IDS is required"},
		{name: "missing agency", missing: "AUTH_EXPECTED_OU", expected: "ExpectedOU is required"},
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
	setRequiredAuthEnv(t)
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
	setRequiredAuthEnv(t)
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
	setRequiredAuthEnv(t)
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
	setRequiredAuthEnv(t)
	t.Setenv("NSW_TOKEN_INSECURE_SKIP_VERIFY", "not-a-bool")

	_, err := LoadConfig()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "invalid value for NSW_TOKEN_INSECURE_SKIP_VERIFY: \"not-a-bool\"" {
		t.Fatalf("unexpected error: %v", err)
	}
}
