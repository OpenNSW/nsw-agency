package auth

import "testing"

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := Config{
		JWKSURL:    "https://localhost:8090/.well-known/jwks.json",
		Issuer:     "https://localhost:8090",
		Audience:   "MY_APP",
		ClientIDs:  []string{"CLIENT_A"},
		ExpectedOU: "fcau",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestConfig_Validate_RequiredFields(t *testing.T) {
	base := Config{
		JWKSURL:    "https://localhost:8090/.well-known/jwks.json",
		Issuer:     "https://localhost:8090",
		Audience:   "MY_APP",
		ClientIDs:  []string{"CLIENT_A"},
		ExpectedOU: "fcau",
	}

	cases := []struct {
		name    string
		mutate  func(c *Config)
		wantErr string
	}{
		{
			name:    "missing JWKS URL",
			mutate:  func(c *Config) { c.JWKSURL = "" },
			wantErr: "AUTH_JWKS_URL is required",
		},
		{
			name:    "invalid JWKS URL — not absolute",
			mutate:  func(c *Config) { c.JWKSURL = "not-a-url" },
			wantErr: "AUTH_JWKS_URL must be a valid absolute URL",
		},
		{
			name:    "invalid JWKS URL — wrong scheme",
			mutate:  func(c *Config) { c.JWKSURL = "ftp://localhost/jwks" },
			wantErr: "AUTH_JWKS_URL must use http or https",
		},
		{
			name:    "missing issuer",
			mutate:  func(c *Config) { c.Issuer = "" },
			wantErr: "AUTH_ISSUER is required",
		},
		{
			name:    "invalid issuer URL",
			mutate:  func(c *Config) { c.Issuer = "not-a-url" },
			wantErr: "AUTH_ISSUER must be a valid absolute URL",
		},
		{
			name:    "missing audience",
			mutate:  func(c *Config) { c.Audience = "" },
			wantErr: "AUTH_AUDIENCE is required",
		},
		{
			name:    "missing client IDs",
			mutate:  func(c *Config) { c.ClientIDs = nil },
			wantErr: "AUTH_CLIENT_IDS is required",
		},
		{
			name:    "empty client IDs slice",
			mutate:  func(c *Config) { c.ClientIDs = []string{} },
			wantErr: "AUTH_CLIENT_IDS is required",
		},
		{
			name:    "missing expected OU",
			mutate:  func(c *Config) { c.ExpectedOU = "" },
			wantErr: "ExpectedOU is required",
		},
		{
			name:    "whitespace-only expected OU",
			mutate:  func(c *Config) { c.ExpectedOU = "   " },
			wantErr: "ExpectedOU is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.mutate(&cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected error %q, got nil", tc.wantErr)
			}
			if err.Error() != tc.wantErr {
				t.Fatalf("expected error %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}
