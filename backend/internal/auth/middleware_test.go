package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	mwKID      = "mw-test-key"
	mwIssuer   = "https://idp.example.com"
	mwAudience = "MW_APP"
	mwClientID = "MW_PORTAL"
	mwOU       = "fcau"
)

// mwGenerateKey generates an RSA key for middleware tests.
func mwGenerateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return key
}

// mwJWKSServer starts an httptest server serving JWKS for the given key.
func mwJWKSServer(t *testing.T, key *rsa.PrivateKey) *httptest.Server {
	t.Helper()
	pub := &key.PublicKey
	body, _ := json.Marshal(jwksResponse{
		Keys: []jwk{{
			Kid: mwKID,
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

// mwExtractor creates a TokenExtractor pointing at the given JWKS server.
func mwExtractor(t *testing.T, jwksURL string) *TokenExtractor {
	t.Helper()
	e, err := NewTokenExtractor(jwksURL, mwIssuer, mwAudience, []string{mwClientID})
	if err != nil {
		t.Fatalf("create token extractor: %v", err)
	}
	return e
}

// mwSign creates a signed RS256 JWT with the given claims.
func mwSign(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = mwKID
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// mwUserClaims returns valid authorization_code claims for middleware tests.
func mwUserClaims(ouHandle string) jwt.MapClaims {
	return jwt.MapClaims{
		"iss":        mwIssuer,
		"aud":        mwAudience,
		"sub":        "sub-mw-001",
		"exp":        time.Now().Add(time.Hour).Unix(),
		"client_id":  mwClientID,
		"grant_type": string(AuthorizationCodeGrant),
		"email":      "user@fcau.gov",
		"ouId":       "ou-123",
		"ouHandle":   ouHandle,
	}
}

// mwClientClaims returns valid client_credentials claims for middleware tests.
func mwClientClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss":        mwIssuer,
		"aud":        mwAudience,
		"exp":        time.Now().Add(time.Hour).Unix(),
		"client_id":  mwClientID,
		"grant_type": string(ClientCredentialsGrant),
	}
}

// mwMockService is a test UserProfileService.
type mwMockService struct {
	returnID  string
	returnErr error
}

func (m *mwMockService) GetOrCreateUser(_, _, _, _, _, _ string) (*string, error) {
	if m.returnErr != nil {
		return nil, m.returnErr
	}
	return &m.returnID, nil
}

// okHandler is a simple handler that records it was called.
func okHandler() (http.Handler, *bool) {
	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	return h, &called
}

// ---------- Middleware tests ----------

func TestMiddleware_NoAuthHeader_PassesThrough(t *testing.T) {
	key := mwGenerateKey(t)
	srv := mwJWKSServer(t, key)
	extractor := mwExtractor(t, srv.URL)

	next, called := okHandler()
	mw := Middleware(nil, extractor, mwOU)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if !*called {
		t.Error("expected next handler to be called when no auth header present")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	// No auth context should be injected
	if ctx := GetAuthContext(req.Context()); ctx != nil {
		t.Error("expected no auth context in request with no header")
	}
}

func TestMiddleware_NilTokenExtractor_Returns500(t *testing.T) {
	next, called := okHandler()
	mw := Middleware(nil, nil, mwOU)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if *called {
		t.Error("expected next handler NOT to be called")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestMiddleware_InvalidToken_Returns401(t *testing.T) {
	key := mwGenerateKey(t)
	srv := mwJWKSServer(t, key)
	extractor := mwExtractor(t, srv.URL)

	next, called := okHandler()
	mw := Middleware(nil, extractor, mwOU)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if *called {
		t.Error("expected next handler NOT to be called")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestMiddleware_WrongOU_Returns403(t *testing.T) {
	key := mwGenerateKey(t)
	srv := mwJWKSServer(t, key)
	extractor := mwExtractor(t, srv.URL)

	next, called := okHandler()
	mw := Middleware(nil, extractor, mwOU) // configured for "fcau"

	// Token claims a different agency
	token := mwSign(t, key, mwUserClaims("npqs"))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if *called {
		t.Error("expected next handler NOT to be called for wrong OU")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestMiddleware_ValidUserToken_NilService_InjectsContextWithEmptyID(t *testing.T) {
	key := mwGenerateKey(t)
	srv := mwJWKSServer(t, key)
	extractor := mwExtractor(t, srv.URL)

	var capturedCtx *AuthContext
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = GetAuthContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	mw := Middleware(nil, extractor, mwOU)

	token := mwSign(t, key, mwUserClaims(mwOU))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if capturedCtx == nil {
		t.Fatal("expected auth context to be injected")
	}
	if capturedCtx.User == nil {
		t.Fatal("expected User in auth context")
	}
	// No service → ID is not populated
	if capturedCtx.User.ID != "" {
		t.Errorf("expected empty User.ID with nil service, got %q", capturedCtx.User.ID)
	}
	if capturedCtx.User.Email != "user@fcau.gov" {
		t.Errorf("expected Email %q, got %q", "user@fcau.gov", capturedCtx.User.Email)
	}
	if capturedCtx.User.OUHandle != mwOU {
		t.Errorf("expected OUHandle %q, got %q", mwOU, capturedCtx.User.OUHandle)
	}
}

func TestMiddleware_ValidUserToken_ServicePopulatesID(t *testing.T) {
	key := mwGenerateKey(t)
	srv := mwJWKSServer(t, key)
	extractor := mwExtractor(t, srv.URL)

	svc := &mwMockService{returnID: "internal-uuid-123"}
	var capturedCtx *AuthContext
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = GetAuthContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	mw := Middleware(svc, extractor, mwOU)

	token := mwSign(t, key, mwUserClaims(mwOU))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if capturedCtx == nil || capturedCtx.User == nil {
		t.Fatal("expected populated auth context")
	}
	if capturedCtx.User.ID != "internal-uuid-123" {
		t.Errorf("expected User.ID %q, got %q", "internal-uuid-123", capturedCtx.User.ID)
	}
}

func TestMiddleware_ValidClientToken_InjectsClientContext(t *testing.T) {
	key := mwGenerateKey(t)
	srv := mwJWKSServer(t, key)
	extractor := mwExtractor(t, srv.URL)

	var capturedCtx *AuthContext
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = GetAuthContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	mw := Middleware(nil, extractor, mwOU)

	token := mwSign(t, key, mwClientClaims())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if capturedCtx == nil || capturedCtx.Client == nil {
		t.Fatal("expected Client in auth context for client credentials token")
	}
	if capturedCtx.Client.ClientID != mwClientID {
		t.Errorf("expected ClientID %q, got %q", mwClientID, capturedCtx.Client.ClientID)
	}
}

// ---------- RequireAuth tests ----------

func TestRequireAuth_NoAuthHeader_Returns401(t *testing.T) {
	key := mwGenerateKey(t)
	srv := mwJWKSServer(t, key)
	extractor := mwExtractor(t, srv.URL)

	next, called := okHandler()
	mw := RequireAuth(nil, extractor, mwOU)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if *called {
		t.Error("expected next handler NOT to be called")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRequireAuth_ValidToken_NilService_Passes(t *testing.T) {
	key := mwGenerateKey(t)
	srv := mwJWKSServer(t, key)
	extractor := mwExtractor(t, srv.URL)

	next, called := okHandler()
	// nil userProfileService — User.ID will be "" but must NOT trigger 403
	mw := RequireAuth(nil, extractor, mwOU)

	token := mwSign(t, key, mwUserClaims(mwOU))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if !*called {
		t.Error("expected next handler to be called when userProfileService is nil")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequireAuth_ServiceProvisioningFails_Returns403(t *testing.T) {
	key := mwGenerateKey(t)
	srv := mwJWKSServer(t, key)
	extractor := mwExtractor(t, srv.URL)

	svc := &mwMockService{returnErr: errors.New("agency mismatch")}
	next, called := okHandler()
	mw := RequireAuth(svc, extractor, mwOU)

	token := mwSign(t, key, mwUserClaims(mwOU))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if *called {
		t.Error("expected next handler NOT to be called when provisioning fails")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestRequireAuth_ValidToken_ServiceSucceeds_Passes(t *testing.T) {
	key := mwGenerateKey(t)
	srv := mwJWKSServer(t, key)
	extractor := mwExtractor(t, srv.URL)

	svc := &mwMockService{returnID: "user-uuid-abc"}
	next, called := okHandler()
	mw := RequireAuth(svc, extractor, mwOU)

	token := mwSign(t, key, mwUserClaims(mwOU))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if !*called {
		t.Error("expected next handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequireAuth_ClientToken_Passes(t *testing.T) {
	key := mwGenerateKey(t)
	srv := mwJWKSServer(t, key)
	extractor := mwExtractor(t, srv.URL)

	next, called := okHandler()
	mw := RequireAuth(nil, extractor, mwOU)

	token := mwSign(t, key, mwClientClaims())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if !*called {
		t.Error("expected next handler to be called for valid client token")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// ---------- GetAuthContext tests ----------

func TestGetAuthContext_ReturnsNilWhenNotSet(t *testing.T) {
	ctx := context.Background()
	if GetAuthContext(ctx) != nil {
		t.Error("expected nil auth context on plain context")
	}
}

func TestGetAuthContext_ReturnsInjectedContext(t *testing.T) {
	expected := &AuthContext{User: &UserContext{ID: "abc"}}
	ctx := context.WithValue(context.Background(), authContextKey, expected)

	got := GetAuthContext(ctx)
	if got == nil {
		t.Fatal("expected auth context, got nil")
	}
	if got.User.ID != "abc" {
		t.Errorf("expected User.ID %q, got %q", "abc", got.User.ID)
	}
}
