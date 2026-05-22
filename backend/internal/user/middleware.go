package user

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

type contextKey string

const userContextKey contextKey = "user"

type jwtClaims struct {
	Sub       string `json:"sub"`
	Email     string `json:"email"`
	GivenName string `json:"given_name"`
	OuHandle  string `json:"ouHandle"`
}

// AuthMiddleware extracts JWT claims from the Authorization header, provisions
// the user via JIT if they do not exist, and injects the UserRecord into the
// request context.
func (s *UserStore) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := extractClaims(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := s.FindOrProvision(claims.Sub, claims.Email, claims.GivenName, claims.OuHandle)
		if err != nil {
			if errors.Is(err, ErrUnauthorizedAgency) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			slog.Error("JIT provisioning failed", "sub", claims.Sub, "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserFromContext retrieves the provisioned UserRecord from the request context.
func UserFromContext(ctx context.Context) (*UserRecord, bool) {
	u, ok := ctx.Value(userContextKey).(*UserRecord)
	return u, ok
}

// extractClaims parses the JWT payload from the Authorization header without
// verifying the signature. Signature verification is expected to be handled
// by the API gateway or a dedicated OIDC middleware layer.
func extractClaims(r *http.Request) (*jwtClaims, error) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("missing bearer token")
	}

	parts := strings.Split(strings.TrimPrefix(authHeader, "Bearer "), ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed jwt")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode jwt payload: %w", err)
	}

	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal jwt claims: %w", err)
	}
	if claims.Sub == "" {
		return nil, fmt.Errorf("jwt missing sub claim")
	}
	if claims.Email == "" {
		return nil, fmt.Errorf("jwt missing email claim")
	}
	if claims.GivenName == "" {
		return nil, fmt.Errorf("jwt missing given_name claim")
	}
	if claims.OuHandle == "" {
		return nil, fmt.Errorf("jwt missing ouHandle claim")
	}

	return &claims, nil
}
