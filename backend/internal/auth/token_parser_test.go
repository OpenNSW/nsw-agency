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
	tpKID      = "tp-test-key"
	tpIssuer   = "https://idp.example.com"
	tpAudience = "TEST_APP"
	tpClientID = "PORTAL_APP"
)

// tpGenerateKey generates an RSA key for token_parser tests.
func tpGenerateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return key
}

// tpJWKSServer starts an httptest server serving JWKS for the given key.
func tpJWKSServer(t *testing.T, key *rsa.PrivateKey) *httptest.Server {
	t.Helper()
	pub := &key.PublicKey
	body, _ := json.Marshal(jwksResponse{
		Keys: []jwk{{
			Kid: tpKID,
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

// tpExtractor creates a TokenExtractor pointing at the given JWKS server.
func tpExtractor(t *testing.T, jwksURL string) *TokenExtractor {
	t.Helper()
	e, err := NewTokenExtractor(jwksURL, tpIssuer, tpAudience, []string{tpClientID})
	if err != nil {
		t.Fatalf("create token extractor: %v", err)
	}
	return e
}

// tpSign creates a signed RS256 JWT with the given claims.
func tpSign(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = tpKID
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// tpUserClaims returns a valid authorization_code claims map.
func tpUserClaims(sub, email, ouID, ouHandle string) jwt.MapClaims {
	return jwt.MapClaims{
		"iss":        tpIssuer,
		"aud":        tpAudience,
		"sub":        sub,
		"exp":        time.Now().Add(time.Hour).Unix(),
		"client_id":  tpClientID,
		"grant_type": string(AuthorizationCodeGrant),
		"email":      email,
		"ouId":       ouID,
		"ouHandle":   ouHandle,
	}
}

// tpClientClaims returns a valid client_credentials claims map.
func tpClientClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss":        tpIssuer,
		"aud":        tpAudience,
		"exp":        time.Now().Add(time.Hour).Unix(),
		"client_id":  tpClientID,
		"grant_type": string(ClientCredentialsGrant),
	}
}

// ---------- Tests ----------

func TestExtractPrincipal_EmptyHeader(t *testing.T) {
	key := tpGenerateKey(t)
	srv := tpJWKSServer(t, key)
	e := tpExtractor(t, srv.URL)

	_, err := e.ExtractPrincipalFromHeader("")
	if err == nil {
		t.Fatal("expected error for empty header, got nil")
	}
}

func TestExtractPrincipal_InvalidHeaderFormat(t *testing.T) {
	key := tpGenerateKey(t)
	srv := tpJWKSServer(t, key)
	e := tpExtractor(t, srv.URL)

	cases := []struct{ name, header string }{
		{"no bearer prefix", "just-a-token"},
		{"basic auth", "Basic dXNlcjpwYXNz"},
		{"extra parts", "Bearer tok1 tok2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := e.ExtractPrincipalFromHeader(tc.header)
			if err == nil {
				t.Fatalf("expected error for header %q, got nil", tc.header)
			}
		})
	}
}

func TestExtractPrincipal_ValidUserToken(t *testing.T) {
	key := tpGenerateKey(t)
	srv := tpJWKSServer(t, key)
	e := tpExtractor(t, srv.URL)

	token := tpSign(t, key, tpUserClaims("sub-001", "user@fcau.gov", "ou-123", "fcau"))
	p, err := e.ExtractPrincipalFromHeader("Bearer " + token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != UserPrincipalType {
		t.Fatalf("expected UserPrincipalType, got %q", p.Type)
	}
	if p.UserPrincipal == nil {
		t.Fatal("expected UserPrincipal to be set")
	}
	if p.UserPrincipal.UserID != "sub-001" {
		t.Errorf("expected UserID %q, got %q", "sub-001", p.UserPrincipal.UserID)
	}
	if p.UserPrincipal.Email != "user@fcau.gov" {
		t.Errorf("expected Email %q, got %q", "user@fcau.gov", p.UserPrincipal.Email)
	}
	if p.UserPrincipal.OUHandle != "fcau" {
		t.Errorf("expected OUHandle %q, got %q", "fcau", p.UserPrincipal.OUHandle)
	}
	if p.UserPrincipal.OUID != "ou-123" {
		t.Errorf("expected OUID %q, got %q", "ou-123", p.UserPrincipal.OUID)
	}
}

func TestExtractPrincipal_UserToken_WithGivenName(t *testing.T) {
	key := tpGenerateKey(t)
	srv := tpJWKSServer(t, key)
	e := tpExtractor(t, srv.URL)

	claims := tpUserClaims("sub-002", "user@fcau.gov", "ou-123", "fcau")
	claims["given_name"] = "Alice"
	token := tpSign(t, key, claims)

	p, err := e.ExtractPrincipalFromHeader("Bearer " + token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.UserPrincipal.GivenName != "Alice" {
		t.Errorf("expected GivenName %q, got %q", "Alice", p.UserPrincipal.GivenName)
	}
}

func TestExtractPrincipal_UserToken_EmptyPhoneNumberAllowed(t *testing.T) {
	key := tpGenerateKey(t)
	srv := tpJWKSServer(t, key)
	e := tpExtractor(t, srv.URL)

	claims := tpUserClaims("sub-003", "user@fcau.gov", "ou-123", "fcau")
	claims["phone_number"] = ""
	token := tpSign(t, key, claims)

	_, err := e.ExtractPrincipalFromHeader("Bearer " + token)
	if err != nil {
		t.Fatalf("empty phone_number should be allowed, got error: %v", err)
	}
}

func TestExtractPrincipal_ValidClientToken(t *testing.T) {
	key := tpGenerateKey(t)
	srv := tpJWKSServer(t, key)
	e := tpExtractor(t, srv.URL)

	token := tpSign(t, key, tpClientClaims())
	p, err := e.ExtractPrincipalFromHeader("Bearer " + token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type != ClientPrincipalType {
		t.Fatalf("expected ClientPrincipalType, got %q", p.Type)
	}
	if p.ClientPrincipal == nil {
		t.Fatal("expected ClientPrincipal to be set")
	}
	if p.ClientPrincipal.ClientID != tpClientID {
		t.Errorf("expected ClientID %q, got %q", tpClientID, p.ClientPrincipal.ClientID)
	}
}

func TestExtractPrincipal_ExpiredToken(t *testing.T) {
	key := tpGenerateKey(t)
	srv := tpJWKSServer(t, key)
	e := tpExtractor(t, srv.URL)

	claims := tpUserClaims("sub-004", "user@fcau.gov", "ou-123", "fcau")
	claims["exp"] = time.Now().Add(-2 * time.Hour).Unix()
	token := tpSign(t, key, claims)

	_, err := e.ExtractPrincipalFromHeader("Bearer " + token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestExtractPrincipal_UnknownClientID(t *testing.T) {
	key := tpGenerateKey(t)
	srv := tpJWKSServer(t, key)
	e := tpExtractor(t, srv.URL)

	claims := tpUserClaims("sub-005", "user@fcau.gov", "ou-123", "fcau")
	claims["client_id"] = "UNKNOWN_CLIENT"
	token := tpSign(t, key, claims)

	_, err := e.ExtractPrincipalFromHeader("Bearer " + token)
	if err == nil {
		t.Fatal("expected error for unknown client_id, got nil")
	}
}

func TestExtractPrincipal_UnsupportedGrantType(t *testing.T) {
	key := tpGenerateKey(t)
	srv := tpJWKSServer(t, key)
	e := tpExtractor(t, srv.URL)

	claims := tpUserClaims("sub-006", "user@fcau.gov", "ou-123", "fcau")
	claims["grant_type"] = "implicit"
	token := tpSign(t, key, claims)

	_, err := e.ExtractPrincipalFromHeader("Bearer " + token)
	if err == nil {
		t.Fatal("expected error for unsupported grant_type, got nil")
	}
}

func TestExtractPrincipal_MissingEmail(t *testing.T) {
	key := tpGenerateKey(t)
	srv := tpJWKSServer(t, key)
	e := tpExtractor(t, srv.URL)

	claims := tpUserClaims("sub-007", "user@fcau.gov", "ou-123", "fcau")
	delete(claims, "email")
	token := tpSign(t, key, claims)

	_, err := e.ExtractPrincipalFromHeader("Bearer " + token)
	if err == nil {
		t.Fatal("expected error for missing email claim, got nil")
	}
}

func TestExtractPrincipal_MissingOUHandle(t *testing.T) {
	key := tpGenerateKey(t)
	srv := tpJWKSServer(t, key)
	e := tpExtractor(t, srv.URL)

	claims := tpUserClaims("sub-008", "user@fcau.gov", "ou-123", "fcau")
	delete(claims, "ouHandle")
	token := tpSign(t, key, claims)

	_, err := e.ExtractPrincipalFromHeader("Bearer " + token)
	if err == nil {
		t.Fatal("expected error for missing ouHandle claim, got nil")
	}
}

func TestExtractPrincipal_WrongSigningKey(t *testing.T) {
	key := tpGenerateKey(t)
	wrongKey := tpGenerateKey(t)
	srv := tpJWKSServer(t, key) // server serves key, but token signed with wrongKey
	e := tpExtractor(t, srv.URL)

	token := tpSign(t, wrongKey, tpUserClaims("sub-009", "user@fcau.gov", "ou-123", "fcau"))

	_, err := e.ExtractPrincipalFromHeader("Bearer " + token)
	if err == nil {
		t.Fatal("expected error for wrong signing key, got nil")
	}
}

func TestExtractPrincipal_WrongIssuer(t *testing.T) {
	key := tpGenerateKey(t)
	srv := tpJWKSServer(t, key)
	e := tpExtractor(t, srv.URL)

	claims := tpUserClaims("sub-010", "user@fcau.gov", "ou-123", "fcau")
	claims["iss"] = "https://evil.example.com"
	token := tpSign(t, key, claims)

	_, err := e.ExtractPrincipalFromHeader("Bearer " + token)
	if err == nil {
		t.Fatal("expected error for wrong issuer, got nil")
	}
}
