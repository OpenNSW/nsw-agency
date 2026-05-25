package auth

import (
	"context"
)

// UserProfileService defines the contract for managing user profiles.
// Implementations are responsible for persisting and managing user records in their system.
//
// This interface is OPTIONAL when using the auth package. If not provided (nil),
// user creation on first login will be skipped. This allows:
//
// 1. Systems that don't track user profiles - just use auth for token validation
// 2. Systems that manage user profiles separately - implement this interface
// 3. Systems that handle user creation elsewhere - pass nil
type UserProfileService interface {
	// GetOrCreateUser creates or retrieves a user profile.
	// Parameters:
	//   - idpUserID: the unique user ID from the identity provider (required)
	//   - email: user's email address (required)
	//   - phone: user's phone number (can be empty)
	//   - organizationID: organization/tenant identifier (required)
	//   - ouHandle: organization unit handle from the identity provider (required)
	//
	// Should be idempotent and must not return an error if the user already exists.
	// Returns the internal user ID of the created or existing user, or an error.
	GetOrCreateUser(idpUserID, email, givenName, phone, organizationID, ouHandle string) (*string, error)
}

// UserContext represents a user principal's runtime context injected into each request.
type UserContext struct {
	ID          string   `json:"id"`
	IDPUserID   string   `json:"idpUserId"`
	Email       string   `json:"email"`
	GivenName   string   `json:"givenName"`
	PhoneNumber string   `json:"phoneNumber"`
	OUID        string   `json:"ouId"`
	OUHandle    string   `json:"ouHandle"`
	Roles       []string `json:"roles"`
}

// ClientContext represents a machine client's context.
type ClientContext struct {
	ClientID string
}

// AuthContext is the transient authentication context injected into each request
// by the auth middleware.
type AuthContext struct {
	User   *UserContext
	Client *ClientContext
}

// ContextKey is a custom type for context keys to avoid collisions.
type ContextKey string

const AuthContextKey ContextKey = "authContext"

// GetAuthContext extracts the AuthContext from a request context.
// Returns nil if no auth context is available.
func GetAuthContext(ctx context.Context) *AuthContext {
	authCtx, ok := ctx.Value(AuthContextKey).(*AuthContext)
	if !ok {
		return nil
	}
	return authCtx
}
