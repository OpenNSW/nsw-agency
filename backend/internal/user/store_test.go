package user

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenNSW/nsw-agency/backend/internal/database"
)

// newTestStore creates an in-memory SQLite UserStore for testing.
func newTestStore(t *testing.T, agency string) *UserStore {
	t.Helper()
	store, err := NewUserStore(database.Config{Driver: "sqlite", SQLite: database.SQLiteConfig{Path: ":memory:"}}, agency)
	if err != nil {
		t.Fatalf("failed to create user store: %v", err)
	}
	if err := store.db.AutoMigrate(&UserRecord{}); err != nil {
		t.Fatalf("failed to migrate users table: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// insertUser seeds a user directly into the test DB without SSOID (simulating
// a pre-seeded user before first login).
func insertUser(t *testing.T, store *UserStore, email, name string) *UserRecord {
	t.Helper()
	u := &UserRecord{Email: email, Name: name}
	if err := store.db.Create(u).Error; err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}
	return u
}

// ---------- 1. Integration Testing: SQLite Connectivity ----------

func TestUserStore_SQLite_FileCreated(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_users.db")

	store, err := NewUserStore(database.Config{Driver: "sqlite", SQLite: database.SQLiteConfig{Path: dbPath}}, "fcau")
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected .db file to be created at configured path")
	}
}

// ---------- 2. Functional Testing: FindAndSync ----------

func TestFindAndSync_UserFound_NoSSID_UpdatesSSID(t *testing.T) {
	store := newTestStore(t, "fcau")
	insertUser(t, store, "user@fcau.gov", "User")

	u, err := store.FindAndSync("sub-001", "user@fcau.gov", "User", "fcau")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.SSOID == nil || *u.SSOID != "sub-001" {
		t.Errorf("expected SSOID to be synced to %q, got %v", "sub-001", u.SSOID)
	}
}

func TestFindAndSync_UserFound_SSIDAlreadySet_NoUpdate(t *testing.T) {
	store := newTestStore(t, "fcau")
	existing := ssoidPtr("existing-sub")
	u := &UserRecord{Email: "user@fcau.gov", Name: "User", SSOID: existing}
	if err := store.db.Create(u).Error; err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	result, err := store.FindAndSync("new-sub", "user@fcau.gov", "User", "fcau")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SSOID == nil || *result.SSOID != "existing-sub" {
		t.Errorf("expected SSOID to remain %q, got %v", "existing-sub", result.SSOID)
	}
}

func TestFindAndSync_UserNotFound_ReturnsError(t *testing.T) {
	store := newTestStore(t, "fcau")

	_, err := store.FindAndSync("sub-001", "unknown@fcau.gov", "Unknown", "fcau")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestFindAndSync_WrongAgency_ReturnsError(t *testing.T) {
	store := newTestStore(t, "fcau")
	insertUser(t, store, "user@npqs.gov", "User")

	_, err := store.FindAndSync("sub-001", "user@npqs.gov", "User", "npqs")
	if !errors.Is(err, ErrUnauthorizedAgency) {
		t.Errorf("expected ErrUnauthorizedAgency, got %v", err)
	}
}

func TestFindAndSync_SyncsName(t *testing.T) {
	store := newTestStore(t, "fcau")
	insertUser(t, store, "user@fcau.gov", "OldName")

	result, err := store.FindAndSync("sub-001", "user@fcau.gov", "NewName", "fcau")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "NewName" {
		t.Errorf("expected Name %q, got %q", "NewName", result.Name)
	}
}

// ---------- 3. Functional Testing: GetOrCreateUser (UserProfileService) ----------

func TestGetOrCreateUser_SeededUser_ReturnsUserID(t *testing.T) {
	store := newTestStore(t, "fcau")
	inserted := insertUser(t, store, "a@fcau.gov", "Alice")

	id, err := store.GetOrCreateUser("sub-010", "a@fcau.gov", "Alice", "", "ou-id", "fcau")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != inserted.UserID {
		t.Errorf("expected UserID %q, got %v", inserted.UserID, id)
	}
}

func TestGetOrCreateUser_UnseededUser_ReturnsError(t *testing.T) {
	store := newTestStore(t, "fcau")

	_, err := store.GetOrCreateUser("sub-011", "notseeded@fcau.gov", "Bob", "", "ou-id", "fcau")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestGetOrCreateUser_WrongAgency_ReturnsError(t *testing.T) {
	store := newTestStore(t, "fcau")

	_, err := store.GetOrCreateUser("sub-012", "c@npqs.gov", "Carol", "", "ou-id", "npqs")
	if !errors.Is(err, ErrUnauthorizedAgency) {
		t.Errorf("expected ErrUnauthorizedAgency, got %v", err)
	}
}

// ---------- 4. Functional Testing: UUID Generation ----------

func TestBeforeCreate_GeneratesUUID(t *testing.T) {
	store := newTestStore(t, "fcau")
	u := insertUser(t, store, "uuid@fcau.gov", "UUID")

	if u.UserID == "" {
		t.Error("expected UserID to be auto-generated")
	}
	if len(u.UserID) != 36 {
		t.Errorf("expected UUID length 36, got %d (%s)", len(u.UserID), u.UserID)
	}
}

func TestBeforeCreate_UniqueUUIDs(t *testing.T) {
	store := newTestStore(t, "fcau")

	u1 := insertUser(t, store, "a@fcau.gov", "A")
	u2 := insertUser(t, store, "b@fcau.gov", "B")

	if u1.UserID == u2.UserID {
		t.Error("expected distinct UUIDs for different users")
	}
}

// ---------- helpers ----------

func ssoidPtr(s string) *string { return &s }
