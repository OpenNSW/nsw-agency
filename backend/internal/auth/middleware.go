package auth

import (
	"context"
	"log/slog"
	"net/http"
)

// Middleware creates an HTTP middleware that extracts and injects authentication context.
//
// Behaviour:
//   - Missing Authorization header: request proceeds without auth context.
//   - Invalid token: request is rejected with 401.
//   - Auth dependencies unavailable: request is rejected with 500.
//   - OUHandle mismatch with expectedOU: request is rejected with 403.
//   - User principal on first login: resolves (get-or-create) user profile if service is provided.
func Middleware(userProfileService UserProfileService, tokenExtractor *TokenExtractor, expectedOU string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				slog.Debug("no authorization header provided")
				next.ServeHTTP(w, r)
				return
			}

			if tokenExtractor == nil {
				slog.Error("auth middleware: token extractor not initialized")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"internal_server_error","message":"authentication subsystem not initialized"}`))
				return
			}

			principal, err := tokenExtractor.ExtractPrincipalFromHeader(authHeader)
			if err != nil {
				slog.Warn("failed to extract principal from token", "error", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized","message":"invalid authentication token"}`))
				return
			}

			if principal == nil || (principal.UserPrincipal == nil && principal.ClientPrincipal == nil) {
				slog.Warn("token extractor returned nil or empty principal")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized","message":"invalid authentication token"}`))
				return
			}

			// Enforce OU check before provisioning — reject cross-agency tokens immediately.
			if principal.UserPrincipal != nil && expectedOU != "" {
				if principal.UserPrincipal.OUHandle != expectedOU {
					slog.Warn("auth: OU handle mismatch", "expected", expectedOU, "got", principal.UserPrincipal.OUHandle)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"error":"forbidden","message":"access denied"}`))
					return
				}
			}

			authCtx := buildAuthContext(principal)
			if principal.UserPrincipal != nil && userProfileService != nil {
				user := principal.UserPrincipal
				userID, err := userProfileService.GetOrCreateUser(
					user.UserID,
					user.Email,
					user.GivenName,
					derefString(user.PhoneNumber),
					user.OUID,
					user.OUHandle,
				)
				if err != nil {
					slog.Error("failed to get or create user profile", "idp_user_id", user.UserID, "error", err)
				} else if userID != nil {
					authCtx.User.ID = *userID
					slog.Debug("resolved user profile", "idp_user_id", user.UserID, "user_id", *userID)
				}
			}

			ctx := context.WithValue(r.Context(), authContextKey, authCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth wraps Middleware and enforces:
//   - 401 if no auth context is present (missing or invalid token)
//   - 403 if the user was authenticated but provisioning failed (e.g. wrong agency)
func RequireAuth(userProfileService UserProfileService, tokenExtractor *TokenExtractor, expectedOU string) func(http.Handler) http.Handler {
	authMiddleware := Middleware(userProfileService, tokenExtractor, expectedOU)
	return func(next http.Handler) http.Handler {
		return authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCtx := GetAuthContext(r.Context())
			if authCtx == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized","message":"authentication required"}`))
				return
			}
			// Valid JWT but provisioning failed — user is not authorised for this agency.
			// Only applies when a userProfileService is active; without one, ID is never populated.
			if authCtx.User != nil && authCtx.User.ID == "" && userProfileService != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"forbidden","message":"access denied"}`))
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}
