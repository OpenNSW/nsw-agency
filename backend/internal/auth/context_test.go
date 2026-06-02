package auth

import (
	"context"
	"testing"
)

// ---------- GetAuthContext ----------

func TestGetAuthContext_NilWhenNotSet(t *testing.T) {
	if got := GetAuthContext(context.Background()); got != nil {
		t.Errorf("expected nil on empty context, got %+v", got)
	}
}

func TestGetAuthContext_NilWhenWrongType(t *testing.T) {
	// Store a string under the key — type assertion should fail gracefully.
	ctx := context.WithValue(context.Background(), authContextKey, "not-an-auth-context")
	if got := GetAuthContext(ctx); got != nil {
		t.Errorf("expected nil when wrong type stored, got %+v", got)
	}
}

func TestGetAuthContext_ReturnsStoredContext(t *testing.T) {
	want := &AuthContext{User: &UserContext{ID: "user-123", Email: "a@fcau.gov"}}
	ctx := context.WithValue(context.Background(), authContextKey, want)

	got := GetAuthContext(ctx)
	if got == nil {
		t.Fatal("expected AuthContext, got nil")
	}
	if got.User.ID != "user-123" {
		t.Errorf("expected User.ID %q, got %q", "user-123", got.User.ID)
	}
	if got.User.Email != "a@fcau.gov" {
		t.Errorf("expected Email %q, got %q", "a@fcau.gov", got.User.Email)
	}
}

func TestGetAuthContext_UserAndClientMutuallyExclusive(t *testing.T) {
	// Verify the struct supports both fields independently.
	userCtx := &AuthContext{User: &UserContext{ID: "u1"}}
	clientCtx := &AuthContext{Client: &ClientContext{ClientID: "c1"}}

	if userCtx.User == nil {
		t.Error("expected User to be set in user context")
	}
	if userCtx.Client != nil {
		t.Error("expected Client to be nil in user context")
	}
	if clientCtx.Client == nil {
		t.Error("expected Client to be set in client context")
	}
	if clientCtx.User != nil {
		t.Error("expected User to be nil in client context")
	}
}

// ---------- buildAuthContext ----------

func TestBuildAuthContext_NilPrincipal(t *testing.T) {
	got := buildAuthContext(nil)
	if got == nil {
		t.Fatal("expected non-nil AuthContext for nil principal")
	}
	if got.User != nil || got.Client != nil {
		t.Error("expected empty AuthContext for nil principal")
	}
}

func TestBuildAuthContext_UserPrincipal(t *testing.T) {
	p := &Principal{
		Type: UserPrincipalType,
		UserPrincipal: &UserPrincipal{
			UserID:    "idp-sub-001",
			Email:     "alice@fcau.gov",
			GivenName: "Alice",
			OUID:      "ou-123",
			OUHandle:  "fcau",
			Roles:     []string{"reviewer"},
		},
	}

	got := buildAuthContext(p)
	if got.User == nil {
		t.Fatal("expected User in AuthContext")
	}
	if got.Client != nil {
		t.Error("expected Client to be nil for user principal")
	}
	if got.User.IDPUserID != "idp-sub-001" {
		t.Errorf("expected IDPUserID %q, got %q", "idp-sub-001", got.User.IDPUserID)
	}
	if got.User.Email != "alice@fcau.gov" {
		t.Errorf("expected Email %q, got %q", "alice@fcau.gov", got.User.Email)
	}
	if got.User.GivenName != "Alice" {
		t.Errorf("expected GivenName %q, got %q", "Alice", got.User.GivenName)
	}
	if got.User.OUHandle != "fcau" {
		t.Errorf("expected OUHandle %q, got %q", "fcau", got.User.OUHandle)
	}
	if got.User.OUID != "ou-123" {
		t.Errorf("expected OUID %q, got %q", "ou-123", got.User.OUID)
	}
	if len(got.User.Roles) != 1 || got.User.Roles[0] != "reviewer" {
		t.Errorf("expected Roles [reviewer], got %v", got.User.Roles)
	}
	// ID is not set by buildAuthContext — middleware sets it after provisioning.
	if got.User.ID != "" {
		t.Errorf("expected empty User.ID before provisioning, got %q", got.User.ID)
	}
}

func TestBuildAuthContext_UserPrincipal_NilInner(t *testing.T) {
	p := &Principal{Type: UserPrincipalType, UserPrincipal: nil}
	got := buildAuthContext(p)
	if got.User != nil || got.Client != nil {
		t.Error("expected empty AuthContext when UserPrincipal is nil")
	}
}

func TestBuildAuthContext_ClientPrincipal(t *testing.T) {
	p := &Principal{
		Type:            ClientPrincipalType,
		ClientPrincipal: &ClientPrincipal{ClientID: "SVC_CLIENT"},
	}

	got := buildAuthContext(p)
	if got.Client == nil {
		t.Fatal("expected Client in AuthContext")
	}
	if got.User != nil {
		t.Error("expected User to be nil for client principal")
	}
	if got.Client.ClientID != "SVC_CLIENT" {
		t.Errorf("expected ClientID %q, got %q", "SVC_CLIENT", got.Client.ClientID)
	}
}

func TestBuildAuthContext_ClientPrincipal_NilInner(t *testing.T) {
	p := &Principal{Type: ClientPrincipalType, ClientPrincipal: nil}
	got := buildAuthContext(p)
	if got.User != nil || got.Client != nil {
		t.Error("expected empty AuthContext when ClientPrincipal is nil")
	}
}

func TestBuildAuthContext_UnknownPrincipalType(t *testing.T) {
	p := &Principal{Type: "unknown"}
	got := buildAuthContext(p)
	if got.User != nil || got.Client != nil {
		t.Error("expected empty AuthContext for unknown principal type")
	}
}

func TestBuildAuthContext_PhoneNumber_DerefedFromPointer(t *testing.T) {
	phone := "+61400000000"
	p := &Principal{
		Type: UserPrincipalType,
		UserPrincipal: &UserPrincipal{
			UserID:      "sub",
			Email:       "b@fcau.gov",
			PhoneNumber: &phone,
		},
	}
	got := buildAuthContext(p)
	if got.User.PhoneNumber != phone {
		t.Errorf("expected PhoneNumber %q, got %q", phone, got.User.PhoneNumber)
	}
}

func TestBuildAuthContext_NilPhoneNumber_DerefedToEmpty(t *testing.T) {
	p := &Principal{
		Type: UserPrincipalType,
		UserPrincipal: &UserPrincipal{
			UserID:      "sub",
			Email:       "b@fcau.gov",
			PhoneNumber: nil,
		},
	}
	got := buildAuthContext(p)
	if got.User.PhoneNumber != "" {
		t.Errorf("expected empty PhoneNumber for nil pointer, got %q", got.User.PhoneNumber)
	}
}
