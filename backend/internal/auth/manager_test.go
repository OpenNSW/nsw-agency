package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	mgKID      = "mg-test-key"
	mgIssuer   = "https://idp.example.com"
	mgAudience = "MG_APP"
	mgClientID = "MG_PORTAL"
	mgOU       = "fcau"
)

// mgGenerateKey generates an RSA key for manager tests.
func mgGenerateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return key
}

// mgJWKSServer starts an httptest server serving JWKS for the given key.
func mgJWKSServer(t *testing.T, key *rsa.PrivateKey) *httptest.Server {
	t.Helper()
	pub := &key.PublicKey
	body, _ := json.Marshal(jwksResponse{
		Keys: []jwk{{
			Kid: mgKID,
			Kty: "RSA",
			Alg: "RS256",
			Use: "sig",
			N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// mgValidConfig returns a valid auth Config pointing at the given JWKS server URL.
func mgValidConfig(jwksURL string) Config {
	return Config{
		JWKSURL:    jwksURL,
		Issuer:     mgIssuer,
		Audience:   mgAudience,
		ClientIDs:  []string{mgClientID},
		ExpectedOU: mgOU,
	}
}

// mgSign creates a signed RS256 JWT.
func mgSign(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = mgKID
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// mgUserClaims returns valid user token claims.
func mgUserClaims(ouHandle string) jwt.MapClaims {
	return jwt.MapClaims{
		"iss":        mgIssuer,
		"aud":        mgAudience,
		"sub":        "sub-mg-001",
		"exp":        time.Now().Add(time.Hour).Unix(),
		"client_id":  mgClientID,
		"grant_type": string(AuthorizationCodeGrant),
		"email":      "user@fcau.gov",
		"ouId":       "ou-123",
		"ouHandle":   ouHandle,
	}
}

// ---------- NewManager ----------

func TestNewManager_ValidConfig_Succeeds(t *testing.T) {
	key := mgGenerateKey(t)
	srv := mgJWKSServer(t, key)

	m, err := NewManager(nil, mgValidConfig(srv.URL))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil Manager")
	}
	_ = m.Close()
}

func TestNewManager_NilUserProfileService_Succeeds(t *testing.T) {
	key := mgGenerateKey(t)
	srv := mgJWKSServer(t, key)

	// nil userProfileService is explicitly supported
	m, err := NewManager(nil, mgValidConfig(srv.URL))
	if err != nil {
		t.Fatalf("expected no error with nil service, got %v", err)
	}
	_ = m.Close()
}

func TestNewManager_InvalidJWKSURL_Fails(t *testing.T) {
	cfg := Config{
		JWKSURL:    "not-a-url",
		Issuer:     mgIssuer,
		Audience:   mgAudience,
		ClientIDs:  []string{mgClientID},
		ExpectedOU: mgOU,
	}
	_, err := NewManager(nil, cfg)
	if err == nil {
		t.Fatal("expected error for invalid JWKS URL, got nil")
	}
}

func TestNewManager_InsecureSkipTLSVerify_Succeeds(t *testing.T) {
	t.Setenv("APP_ENV", "development") // insecure TLS is only honored in development
	key := mgGenerateKey(t)
	srv := mgJWKSServer(t, key)

	cfg := mgValidConfig(srv.URL)
	cfg.InsecureSkipTLSVerify = true

	m, err := NewManager(nil, cfg)
	if err != nil {
		t.Fatalf("expected no error with InsecureSkipTLSVerify, got %v", err)
	}
	_ = m.Close()
}

func TestNewManager_InsecureSkipTLSVerify_FailsClosedOutsideDev(t *testing.T) {
	t.Setenv("APP_ENV", "production") // not development -> must refuse to start
	cfg := mgValidConfig("https://idp.example.com/oauth2/jwks")
	cfg.InsecureSkipTLSVerify = true

	if _, err := NewManager(nil, cfg); err == nil {
		t.Fatalf("expected NewManager to fail closed when InsecureSkipTLSVerify is set outside development")
	}
}

// ---------- Health ----------

func TestManager_Health_ReturnsNilWhenInitialized(t *testing.T) {
	key := mgGenerateKey(t)
	srv := mgJWKSServer(t, key)

	m, err := NewManager(nil, mgValidConfig(srv.URL))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer func() { _ = m.Close() }()

	if err := m.Health(); err != nil {
		t.Errorf("expected Health() to return nil, got %v", err)
	}
}

// ---------- Close ----------

func TestManager_Close_ReturnsNil(t *testing.T) {
	key := mgGenerateKey(t)
	srv := mgJWKSServer(t, key)

	m, err := NewManager(nil, mgValidConfig(srv.URL))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Errorf("expected Close() to return nil, got %v", err)
	}
}

// ---------- RequireAuthMiddleware ----------

func TestManager_RequireAuthMiddleware_NoHeader_Returns401(t *testing.T) {
	key := mgGenerateKey(t)
	srv := mgJWKSServer(t, key)

	m, err := NewManager(nil, mgValidConfig(srv.URL))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer func() { _ = m.Close() }()

	protect := m.RequireAuthMiddleware()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	protect(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestManager_RequireAuthMiddleware_ValidToken_Passes(t *testing.T) {
	key := mgGenerateKey(t)
	srv := mgJWKSServer(t, key)

	m, err := NewManager(nil, mgValidConfig(srv.URL))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer func() { _ = m.Close() }()

	protect := m.RequireAuthMiddleware()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	token := mgSign(t, key, mgUserClaims(mgOU))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	protect(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestManager_RequireAuthMiddleware_WrongOU_Returns403(t *testing.T) {
	key := mgGenerateKey(t)
	srv := mgJWKSServer(t, key)

	m, err := NewManager(nil, mgValidConfig(srv.URL)) // configured for "fcau"
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer func() { _ = m.Close() }()

	protect := m.RequireAuthMiddleware()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	token := mgSign(t, key, mgUserClaims("npqs")) // token claims wrong agency
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	protect(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// ---------- OptionalAuthMiddleware ----------

func TestManager_OptionalAuthMiddleware_NoHeader_PassesThrough(t *testing.T) {
	key := mgGenerateKey(t)
	srv := mgJWKSServer(t, key)

	m, err := NewManager(nil, mgValidConfig(srv.URL))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer func() { _ = m.Close() }()

	optional := m.OptionalAuthMiddleware()
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	optional(next).ServeHTTP(rr, req)

	if !called {
		t.Error("expected next handler to be called with no auth header")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestManager_OptionalAuthMiddleware_ValidToken_InjectsContext(t *testing.T) {
	key := mgGenerateKey(t)
	srv := mgJWKSServer(t, key)

	m, err := NewManager(nil, mgValidConfig(srv.URL))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer func() { _ = m.Close() }()

	optional := m.OptionalAuthMiddleware()
	var capturedCtx *AuthContext
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = GetAuthContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	token := mgSign(t, key, mgUserClaims(mgOU))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	optional(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if capturedCtx == nil || capturedCtx.User == nil {
		t.Fatal("expected auth context with User to be injected")
	}
	if capturedCtx.User.Email != "user@fcau.gov" {
		t.Errorf("expected Email %q, got %q", "user@fcau.gov", capturedCtx.User.Email)
	}
}
