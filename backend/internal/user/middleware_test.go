package user

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// makeTestToken encodes claims as a structurally valid JWT (header.payload.signature).
// The signature is fake — sufficient for claim extraction tests only.
func makeTestToken(claims map[string]any) string {
	payload, _ := json.Marshal(claims)
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return "Bearer eyJhbGciOiJSUzI1NiJ9." + encoded + ".fakesignature"
}

// validClaims returns a complete set of claims matching the IDP token format.
func validClaims() map[string]any {
	return map[string]any{
		"sub":        "019e4f60-58f9-706c-af53-4174e32fe6a3",
		"email":      "admin@fcau.gov",
		"given_name": "Admin",
		"ouHandle":   "fcau",
	}
}

// ---------- 1. Unit Testing: extractClaims ----------

func TestExtractClaims_MissingAuthHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := extractClaims(r)
	if err == nil {
		t.Error("expected error for missing Authorization header")
	}
}

func TestExtractClaims_NotBearer(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	_, err := extractClaims(r)
	if err == nil {
		t.Error("expected error for non-bearer scheme")
	}
}

func TestExtractClaims_MalformedJWT(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer only.two")
	_, err := extractClaims(r)
	if err == nil {
		t.Error("expected error for JWT with wrong number of segments")
	}
}

func TestExtractClaims_InvalidBase64Payload(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer header.!!!invalid!!!.signature")
	_, err := extractClaims(r)
	if err == nil {
		t.Error("expected error for invalid base64 payload")
	}
}

func TestExtractClaims_MissingSub(t *testing.T) {
	claims := validClaims()
	delete(claims, "sub")
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", makeTestToken(claims))
	_, err := extractClaims(r)
	if err == nil {
		t.Error("expected error for missing sub claim")
	}
}

func TestExtractClaims_MissingEmail(t *testing.T) {
	claims := validClaims()
	delete(claims, "email")
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", makeTestToken(claims))
	_, err := extractClaims(r)
	if err == nil {
		t.Error("expected error for missing email claim")
	}
}

func TestExtractClaims_MissingGivenName(t *testing.T) {
	claims := validClaims()
	delete(claims, "given_name")
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", makeTestToken(claims))
	_, err := extractClaims(r)
	if err == nil {
		t.Error("expected error for missing given_name claim")
	}
}

func TestExtractClaims_MissingOuHandle(t *testing.T) {
	claims := validClaims()
	delete(claims, "ouHandle")
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", makeTestToken(claims))
	_, err := extractClaims(r)
	if err == nil {
		t.Error("expected error for missing ouHandle claim")
	}
}

func TestExtractClaims_Valid(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", makeTestToken(validClaims()))

	claims, err := extractClaims(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Sub != "019e4f60-58f9-706c-af53-4174e32fe6a3" {
		t.Errorf("unexpected sub: %q", claims.Sub)
	}
	if claims.Email != "admin@fcau.gov" {
		t.Errorf("unexpected email: %q", claims.Email)
	}
	if claims.GivenName != "Admin" {
		t.Errorf("unexpected given_name: %q", claims.GivenName)
	}
	if claims.OuHandle != "fcau" {
		t.Errorf("unexpected ouHandle: %q", claims.OuHandle)
	}
}

// ---------- 2. Integration Testing: AuthMiddleware ----------

func TestAuthMiddleware_ValidToken_ProvisionAndPass(t *testing.T) {
	store := newTestStore(t, "fcau")

	var capturedUser *UserRecord
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser, _ = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", makeTestToken(validClaims()))
	w := httptest.NewRecorder()

	store.AuthMiddleware(next).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if capturedUser == nil {
		t.Fatal("expected user to be injected into context")
	}
	if capturedUser.SSOID != "019e4f60-58f9-706c-af53-4174e32fe6a3" {
		t.Errorf("unexpected SSOID in context: %q", capturedUser.SSOID)
	}
}

func TestAuthMiddleware_MissingToken_Returns401(t *testing.T) {
	store := newTestStore(t, "fcau")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	store.AuthMiddleware(next).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_WrongAgency_Returns403(t *testing.T) {
	store := newTestStore(t, "fcau")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})

	claims := validClaims()
	claims["ouHandle"] = "npqs"

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", makeTestToken(claims))
	w := httptest.NewRecorder()

	store.AuthMiddleware(next).ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestAuthMiddleware_SecondRequest_ExistingUser(t *testing.T) {
	store := newTestStore(t, "fcau")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// First request — provisions the user.
	r1 := httptest.NewRequest(http.MethodGet, "/", nil)
	r1.Header.Set("Authorization", makeTestToken(validClaims()))
	store.AuthMiddleware(next).ServeHTTP(httptest.NewRecorder(), r1)

	// Second request — retrieves the existing user without error.
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.Header.Set("Authorization", makeTestToken(validClaims()))
	w2 := httptest.NewRecorder()
	store.AuthMiddleware(next).ServeHTTP(w2, r2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200 on second request, got %d", w2.Code)
	}
}

// ---------- 3. Unit Testing: UserFromContext ----------

func TestUserFromContext_NotSet(t *testing.T) {
	_, ok := UserFromContext(context.Background())
	if ok {
		t.Error("expected false for empty context")
	}
}

func TestUserFromContext_Set(t *testing.T) {
	expected := &UserRecord{UserID: "test-id", SSOID: "sub-test"}
	ctx := context.WithValue(context.Background(), userContextKey, expected)

	u, ok := UserFromContext(ctx)
	if !ok {
		t.Fatal("expected user to be found in context")
	}
	if u.UserID != expected.UserID {
		t.Errorf("expected UserID %q, got %q", expected.UserID, u.UserID)
	}
}
