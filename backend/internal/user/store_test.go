package user

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenNSW/nsw-agency/backend/internal/config"
	"github.com/OpenNSW/nsw-agency/backend/internal/database"
)

func newTestStore(t *testing.T, agency string) *UserStore {
	t.Helper()
	store, err := NewUserStore(config.Config{
		DB:     database.Config{Driver: "sqlite", Path: ":memory:"},
		Agency: agency,
	})
	if err != nil {
		t.Fatalf("failed to create user store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// ---------- 1. Integration Testing: SQLite Connectivity ----------

func TestUserStore_SQLite_FileCreated(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_users.db")

	_, err := NewUserStore(config.Config{
		DB:     database.Config{Driver: "sqlite", Path: dbPath},
		Agency: "fcau",
	})
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected .db file to be created at configured path")
	}
}

func TestUserStore_SQLite_SchemaMigration(t *testing.T) {
	store := newTestStore(t, "fcau")
	if !store.db.Migrator().HasTable(&UserRecord{}) {
		t.Error("users table was not created after migration")
	}
}

// ---------- 2. Functional Testing: FindOrProvision ----------

func TestFindOrProvision_NewUser(t *testing.T) {
	store := newTestStore(t, "fcau")

	u, err := store.FindOrProvision("sub-001", "admin@fcau.gov", "Admin", "fcau")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.UserID == "" {
		t.Error("expected UserID to be generated")
	}
	if u.SSOID != "sub-001" {
		t.Errorf("expected SSOID %q, got %q", "sub-001", u.SSOID)
	}
	if u.Email != "admin@fcau.gov" {
		t.Errorf("expected Email %q, got %q", "admin@fcau.gov", u.Email)
	}
	if u.Name != "Admin" {
		t.Errorf("expected Name %q, got %q", "Admin", u.Name)
	}
}

func TestFindOrProvision_WrongAgency(t *testing.T) {
	store := newTestStore(t, "fcau")

	_, err := store.FindOrProvision("sub-002", "officer@npqs.gov", "Officer", "npqs")
	if !errors.Is(err, ErrUnauthorizedAgency) {
		t.Errorf("expected ErrUnauthorizedAgency, got %v", err)
	}
}

func TestFindOrProvision_ExistingUser_NoChange(t *testing.T) {
	store := newTestStore(t, "fcau")

	first, err := store.FindOrProvision("sub-003", "user@fcau.gov", "User", "fcau")
	if err != nil {
		t.Fatalf("unexpected error on first provision: %v", err)
	}

	second, err := store.FindOrProvision("sub-003", "user@fcau.gov", "User", "fcau")
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if first.UserID != second.UserID {
		t.Errorf("expected same UserID, got %q and %q", first.UserID, second.UserID)
	}
}

func TestFindOrProvision_ExistingUser_SyncsAttributes(t *testing.T) {
	store := newTestStore(t, "fcau")

	_, err := store.FindOrProvision("sub-004", "old@fcau.gov", "OldName", "fcau")
	if err != nil {
		t.Fatalf("unexpected error on initial provision: %v", err)
	}

	updated, err := store.FindOrProvision("sub-004", "new@fcau.gov", "NewName", "fcau")
	if err != nil {
		t.Fatalf("unexpected error on attribute sync: %v", err)
	}
	if updated.Email != "new@fcau.gov" {
		t.Errorf("expected Email %q, got %q", "new@fcau.gov", updated.Email)
	}
	if updated.Name != "NewName" {
		t.Errorf("expected Name %q, got %q", "NewName", updated.Name)
	}
}

func TestFindOrProvision_ExistingUser_AgencyCheckSkipped(t *testing.T) {
	// Agency check only blocks NEW users. Existing users pass through regardless
	// of ouHandle since they were already validated at provisioning time.
	store := newTestStore(t, "fcau")

	_, err := store.FindOrProvision("sub-005", "officer@fcau.gov", "Officer", "fcau")
	if err != nil {
		t.Fatalf("unexpected error on initial provision: %v", err)
	}

	u, err := store.FindOrProvision("sub-005", "officer@fcau.gov", "Officer", "wrong-agency")
	if err != nil {
		t.Errorf("expected success for existing user regardless of ouHandle, got: %v", err)
	}
	if u == nil {
		t.Error("expected user to be returned")
	}
}

// ---------- 3. Functional Testing: UUID Generation ----------

func TestBeforeCreate_GeneratesUUID(t *testing.T) {
	store := newTestStore(t, "fcau")

	u, err := store.FindOrProvision("sub-006", "uuid@fcau.gov", "UUID", "fcau")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.UserID == "" {
		t.Error("expected UserID to be auto-generated")
	}
	// UUID v4: 8-4-4-4-12 hex chars with dashes = 36 characters
	if len(u.UserID) != 36 {
		t.Errorf("expected UUID length 36, got %d (%s)", len(u.UserID), u.UserID)
	}
}

func TestBeforeCreate_UniqueUUIDs(t *testing.T) {
	store := newTestStore(t, "fcau")

	u1, _ := store.FindOrProvision("sub-007", "a@fcau.gov", "A", "fcau")
	u2, _ := store.FindOrProvision("sub-008", "b@fcau.gov", "B", "fcau")

	if u1.UserID == u2.UserID {
		t.Error("expected distinct UUIDs for different users")
	}
}
