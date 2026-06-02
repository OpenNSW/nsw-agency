package auth

import (
	"fmt"
	"net/url"
	"strings"
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
	if err := validateHTTPURL("AUTH_JWKS_URL", c.JWKSURL); err != nil {
		return err
	}
	if c.Issuer == "" {
		return fmt.Errorf("AUTH_ISSUER is required")
	}
	if err := validateHTTPURL("AUTH_ISSUER", c.Issuer); err != nil {
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

func validateHTTPURL(name, value string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be a valid absolute URL", name)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", name)
	}
	return nil
}
