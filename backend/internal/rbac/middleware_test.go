package rbac

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenNSW/nsw-agency/backend/internal/taskconfig"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------- Unit tests: resolveAllowedActions ----------

func TestResolveAllowedActions_NoRoles(t *testing.T) {
	permissions := []taskconfig.Permission{
		{Role: "lab_officer", Actions: []string{"VIEW", "REVIEW"}},
	}
	actions := resolveAllowedActions(nil, permissions)
	if len(actions) != 0 {
		t.Errorf("expected no actions, got %v", actions)
	}
}

func TestResolveAllowedActions_MatchingRole(t *testing.T) {
	roles := []RoleRecord{{Name: "lab_officer"}}
	permissions := []taskconfig.Permission{
		{Role: "lab_officer", Actions: []string{"VIEW", "REVIEW"}},
	}
	actions := resolveAllowedActions(roles, permissions)
	if len(actions) != 2 {
		t.Errorf("expected 2 actions, got %v", actions)
	}
}

func TestResolveAllowedActions_MultipleRoles_Union(t *testing.T) {
	roles := []RoleRecord{
		{Name: "lab_officer"},
		{Name: "lab_manager"},
	}
	permissions := []taskconfig.Permission{
		{Role: "lab_officer", Actions: []string{"VIEW"}},
		{Role: "lab_manager", Actions: []string{"VIEW", "REVIEW"}},
	}
	// VIEW appears in both roles but should be deduplicated — expect 2 unique actions.
	actions := resolveAllowedActions(roles, permissions)
	if len(actions) != 2 {
		t.Errorf("expected 2 unique actions, got %v", actions)
	}
}

func TestResolveAllowedActions_NoMatch(t *testing.T) {
	roles := []RoleRecord{{Name: "unrelated_role"}}
	permissions := []taskconfig.Permission{
		{Role: "lab_officer", Actions: []string{"VIEW"}},
	}
	actions := resolveAllowedActions(roles, permissions)
	if len(actions) != 0 {
		t.Errorf("expected no actions, got %v", actions)
	}
}

func TestResolveAllowedActions_EmptyPermissions(t *testing.T) {
	roles := []RoleRecord{{Name: "lab_officer"}}
	actions := resolveAllowedActions(roles, nil)
	if len(actions) != 0 {
		t.Errorf("expected no actions for empty permissions, got %v", actions)
	}
}

// ---------- Unit tests: hasAction ----------

func TestHasAction_Present(t *testing.T) {
	if !hasAction([]string{"VIEW", "REVIEW"}, "VIEW") {
		t.Error("expected hasAction to return true for VIEW")
	}
}

func TestHasAction_Absent(t *testing.T) {
	if hasAction([]string{"VIEW"}, "REVIEW") {
		t.Error("expected hasAction to return false for REVIEW")
	}
}

func TestHasAction_EmptySlice(t *testing.T) {
	if hasAction(nil, "VIEW") {
		t.Error("expected hasAction to return false for empty actions")
	}
}

// ---------- Helpers ----------

type mockTaskCodeResolver struct {
	taskCode string
	err      error
}

func (m *mockTaskCodeResolver) GetTaskCode(_ context.Context, _ string) (string, error) {
	return m.taskCode, m.err
}

type mockTaskConfigProvider struct {
	cfg *taskconfig.TaskConfig
	err error
}

func (m *mockTaskConfigProvider) GetTaskConfig(_ string) (*taskconfig.TaskConfig, error) {
	return m.cfg, m.err
}

func newMiddlewareTestDB(t *testing.T) *UserRoleStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&RoleRecord{}, &UserRoleRecord{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	return &UserRoleStore{db: db}
}

// ---------- Integration tests: RequireAction ----------

func TestRequireAction_NoPermissionsInConfig_Allows(t *testing.T) {
	urs := newMiddlewareTestDB(t)
	m := NewMiddleware(urs,
		&mockTaskCodeResolver{taskCode: "fcau_lab_test_v1"},
		&mockTaskConfigProvider{cfg: &taskconfig.TaskConfig{
			TaskCode:    "fcau_lab_test_v1",
			Permissions: nil,
		}},
	)

	called := false
	handler := m.RequireAction("VIEW")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("taskId", "task-1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if !called {
		t.Error("expected handler to be called when no permissions are defined")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
