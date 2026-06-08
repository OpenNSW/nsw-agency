package rbac

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenNSW/nsw-agency/backend/internal/auth"
	"github.com/OpenNSW/nsw-agency/backend/internal/taskconfig"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------- Unit tests: ResolveAccess ----------

func TestResolveAccess_NoRoles(t *testing.T) {
	permissions := []taskconfig.Permission{
		{Role: "lab_officer", Actions: []string{"VIEW", "REVIEW"}},
	}
	accessible, actions := ResolveAccess(nil, permissions)
	if accessible {
		t.Error("expected isAccessible false when user has no roles")
	}
	if len(actions) != 0 {
		t.Errorf("expected no actions, got %v", actions)
	}
}

func TestResolveAccess_MatchingRole(t *testing.T) {
	roles := []RoleRecord{{Name: "lab_officer"}}
	permissions := []taskconfig.Permission{
		{Role: "lab_officer", Actions: []string{"VIEW", "REVIEW"}},
	}
	accessible, actions := ResolveAccess(roles, permissions)
	if !accessible {
		t.Error("expected isAccessible true when role matches")
	}
	if len(actions) != 2 {
		t.Errorf("expected 2 actions, got %v", actions)
	}
}

func TestResolveAccess_MultipleRoles_Union(t *testing.T) {
	roles := []RoleRecord{
		{Name: "lab_officer"},
		{Name: "lab_manager"},
	}
	permissions := []taskconfig.Permission{
		{Role: "lab_officer", Actions: []string{"VIEW"}},
		{Role: "lab_manager", Actions: []string{"VIEW", "REVIEW"}},
	}
	// VIEW appears in both roles but should be deduplicated — expect 2 unique actions.
	_, actions := ResolveAccess(roles, permissions)
	if len(actions) != 2 {
		t.Errorf("expected 2 unique actions, got %v", actions)
	}
}

func TestResolveAccess_NoMatch(t *testing.T) {
	roles := []RoleRecord{{Name: "unrelated_role"}}
	permissions := []taskconfig.Permission{
		{Role: "lab_officer", Actions: []string{"VIEW"}},
	}
	accessible, actions := ResolveAccess(roles, permissions)
	if accessible {
		t.Error("expected isAccessible false when no role matches")
	}
	if len(actions) != 0 {
		t.Errorf("expected no actions, got %v", actions)
	}
}

func TestResolveAccess_EmptyPermissions(t *testing.T) {
	roles := []RoleRecord{{Name: "lab_officer"}}
	accessible, actions := ResolveAccess(roles, nil)
	if accessible {
		t.Error("expected isAccessible false for empty permissions")
	}
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

func newMiddlewareTestDB(t *testing.T) *RoleService {
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
	return NewRoleService(db)
}

// ---------- Integration tests: RequireAction ----------

func TestRequireAction_NoPermissionsInConfig_Allows(t *testing.T) {
	svc := newMiddlewareTestDB(t)
	m := NewMiddleware(svc,
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

func TestRequireAction_UserHasRole_Allows(t *testing.T) {
	svc := newMiddlewareTestDB(t)

	role, err := svc.Create("lab_officer")
	if err != nil {
		t.Fatalf("failed to create role: %v", err)
	}
	const testUserID = "user-001"
	if err := svc.Assign(testUserID, role.ID); err != nil {
		t.Fatalf("failed to assign role: %v", err)
	}

	m := NewMiddleware(svc,
		&mockTaskCodeResolver{taskCode: "fcau_lab_test_v1"},
		&mockTaskConfigProvider{cfg: &taskconfig.TaskConfig{
			TaskCode:    "fcau_lab_test_v1",
			Permissions: []taskconfig.Permission{{Role: "lab_officer", Actions: []string{"VIEW", "REVIEW"}}},
		}},
	)

	called := false
	handler := m.RequireAction("VIEW")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("taskId", "task-1")
	r = r.WithContext(auth.WithAuthContext(r.Context(), &auth.AuthContext{
		User: &auth.UserContext{ID: testUserID},
	}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if !called {
		t.Error("expected handler to be called when user has required role")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequireAction_UserLacksRole_Forbidden(t *testing.T) {
	svc := newMiddlewareTestDB(t)

	m := NewMiddleware(svc,
		&mockTaskCodeResolver{taskCode: "fcau_lab_test_v1"},
		&mockTaskConfigProvider{cfg: &taskconfig.TaskConfig{
			TaskCode:    "fcau_lab_test_v1",
			Permissions: []taskconfig.Permission{{Role: "lab_officer", Actions: []string{"VIEW"}}},
		}},
	)

	handler := m.RequireAction("VIEW")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("taskId", "task-1")
	r = r.WithContext(auth.WithAuthContext(r.Context(), &auth.AuthContext{
		User: &auth.UserContext{ID: "user-no-roles"},
	}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRequireAction_NoAuthContext_Unauthorized(t *testing.T) {
	svc := newMiddlewareTestDB(t)

	m := NewMiddleware(svc,
		&mockTaskCodeResolver{taskCode: "fcau_lab_test_v1"},
		&mockTaskConfigProvider{cfg: &taskconfig.TaskConfig{
			TaskCode:    "fcau_lab_test_v1",
			Permissions: []taskconfig.Permission{{Role: "lab_officer", Actions: []string{"VIEW"}}},
		}},
	)

	handler := m.RequireAction("VIEW")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("taskId", "task-1")
	// No auth context injected.
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequireAction_ResolverError_InternalServerError(t *testing.T) {
	svc := newMiddlewareTestDB(t)

	m := NewMiddleware(svc,
		&mockTaskCodeResolver{err: fmt.Errorf("db unavailable")},
		&mockTaskConfigProvider{},
	)

	handler := m.RequireAction("VIEW")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("taskId", "task-1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
