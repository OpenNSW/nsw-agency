package auth

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Manager handles all authentication-related operations and middleware setup.
// NewManager is the single entry point for auth initialisation in the application.
// userProfileService is optional — pass nil to disable JIT user creation.
type Manager struct {
	userProfileService UserProfileService
	tokenExtractor     *TokenExtractor
	authConfig         Config
	middleware         func(http.Handler) http.Handler
}

// NewManager creates and initialises a new auth manager.
func NewManager(userProfileService UserProfileService, authConfig Config) (*Manager, error) {
	slog.Info("initializing auth manager", "user_profile_service_enabled", userProfileService != nil)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	if authConfig.InsecureSkipTLSVerify {
		if tr, ok := http.DefaultTransport.(*http.Transport); ok {
			customTransport := tr.Clone()
			customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
			httpClient.Transport = customTransport
		}
	}

	tokenExtractor, err := NewTokenExtractorWithClient(
		authConfig.JWKSURL, authConfig.Issuer, authConfig.Audience, authConfig.ClientIDs, httpClient,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize token extractor: %w", err)
	}
	if tokenExtractor == nil {
		return nil, fmt.Errorf("token extractor not initialized")
	}

	return &Manager{
		userProfileService: userProfileService,
		tokenExtractor:     tokenExtractor,
		authConfig:         authConfig,
		middleware:         Middleware(userProfileService, tokenExtractor, authConfig.ExpectedOU),
	}, nil
}

// RequireAuthMiddleware returns a middleware that rejects unauthenticated requests with 401.
func (m *Manager) RequireAuthMiddleware() func(http.Handler) http.Handler {
	return RequireAuth(m.userProfileService, m.tokenExtractor, m.authConfig.ExpectedOU)
}

// OptionalAuthMiddleware returns a middleware that injects auth context when present
// but allows requests through without one.
func (m *Manager) OptionalAuthMiddleware() func(http.Handler) http.Handler {
	return m.middleware
}

// Health checks that the auth system is properly initialised.
func (m *Manager) Health() error {
	if m.tokenExtractor == nil {
		return fmt.Errorf("token extractor not initialized")
	}
	slog.Info("auth system health check passed", "user_profile_service_enabled", m.userProfileService != nil)
	return nil
}

// Close performs cleanup for the auth manager.
func (m *Manager) Close() error {
	slog.Debug("auth manager closing")
	return nil
}
