package nswclient

import (
	"fmt"
	"strings"
)

// Config holds the connection and OAuth2 credentials for the NSW service.
type Config struct {
	BaseURL                 string
	ClientID                string
	ClientSecret            string
	TokenURL                string
	Scopes                  []string
	TokenInsecureSkipVerify bool
}

// Validate ensures the required OAuth2 connection fields are present.
func (c Config) Validate() error {
	if strings.TrimSpace(c.BaseURL) == "" {
		return fmt.Errorf("NSW_API_BASE_URL is required")
	}
	if strings.TrimSpace(c.ClientID) == "" {
		return fmt.Errorf("NSW_CLIENT_ID is required")
	}
	if strings.TrimSpace(c.ClientSecret) == "" {
		return fmt.Errorf("NSW_CLIENT_SECRET is required")
	}
	if strings.TrimSpace(c.TokenURL) == "" {
		return fmt.Errorf("NSW_TOKEN_URL is required")
	}
	return nil
}
