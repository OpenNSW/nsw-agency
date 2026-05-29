package auth

import (
	"fmt"
	"strings"

	"github.com/OpenNSW/nsw-agency/backend/internal/validation"
)

type Config struct {
	JWKSURL               string
	Issuer                string
	Audience              string
	ClientIDs             []string
	InsecureSkipTLSVerify bool
	ExpectedOU            string
}

func (c Config) Validate() error {
	if c.JWKSURL == "" {
		return fmt.Errorf("AUTH_JWKS_URL is required")
	}
	if err := validation.HTTPURL("AUTH_JWKS_URL", c.JWKSURL); err != nil {
		return err
	}
	if c.Issuer == "" {
		return fmt.Errorf("AUTH_ISSUER is required")
	}
	if err := validation.HTTPURL("AUTH_ISSUER", c.Issuer); err != nil {
		return err
	}
	if c.Audience == "" {
		return fmt.Errorf("AUTH_AUDIENCE is required")
	}
	if len(c.ClientIDs) == 0 {
		return fmt.Errorf("AUTH_CLIENT_IDS is required")
	}
	if strings.TrimSpace(c.ExpectedOU) == "" {
		return fmt.Errorf("ExpectedOU is required")
	}
	return nil
}
