package rbac

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/OpenNSW/nsw-agency/backend/internal/auth"
	"github.com/OpenNSW/nsw-agency/backend/internal/taskconfig"
	"github.com/OpenNSW/nsw-agency/backend/pkg/httputil"
)

// TaskCodeResolver resolves a task's task_code from its task_id.
type TaskCodeResolver interface {
	GetTaskCode(ctx context.Context, taskID string) (string, error)
}

// TaskConfigProvider retrieves a TaskConfig by task_code.
type TaskConfigProvider interface {
	GetTaskConfig(taskCode string) (*taskconfig.TaskConfig, error)
}

// Middleware enforces role-based access control on task routes.
type Middleware struct {
	roleService      *RoleService
	taskCodeResolver TaskCodeResolver
	configProvider   TaskConfigProvider
}

// NewMiddleware creates a new RBAC Middleware.
func NewMiddleware(roleService *RoleService, taskCodeResolver TaskCodeResolver, configProvider TaskConfigProvider) *Middleware {
	return &Middleware{
		roleService:      roleService,
		taskCodeResolver: taskCodeResolver,
		configProvider:   configProvider,
	}
}

// RequireAction returns middleware that enforces the given action is permitted
// for the authenticated user on the requested task. If the task config defines
// no permissions, all authenticated users are allowed (current behaviour preserved).
func (m *Middleware) RequireAction(action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			taskID := r.PathValue("taskId")
			if taskID == "" {
				httputil.WriteJSONError(w, http.StatusBadRequest, "taskId is required")
				return
			}

			taskCode, err := m.taskCodeResolver.GetTaskCode(ctx, taskID)
			if err != nil {
				slog.ErrorContext(ctx, "rbac: failed to resolve task code", "taskId", taskID, "error", err)
				httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to resolve task")
				return
			}

			cfg, err := m.configProvider.GetTaskConfig(taskCode)
			if err != nil || cfg == nil || len(cfg.Permissions) == 0 {
				// No permissions defined — preserve current behaviour, allow all authenticated users.
				next.ServeHTTP(w, r)
				return
			}

			authCtx := auth.GetAuthContext(ctx)
			if authCtx == nil || authCtx.User == nil {
				httputil.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			roles, err := m.roleService.GetRolesForUser(authCtx.User.ID)
			if err != nil {
				slog.ErrorContext(ctx, "rbac: failed to get roles for user", "userID", authCtx.User.ID, "error", err)
				httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to resolve user roles")
				return
			}

			_, allowedActions := ResolveAccess(roles, cfg.Permissions)
			if !hasAction(allowedActions, action) {
				httputil.WriteJSONError(w, http.StatusForbidden, "access denied")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ResolveAccess returns whether the user has access to the task (isAccessible)
// and the union of actions they may perform, based on their roles and the task's
// permission configuration. Returns (false, nil) when no role matches.
func ResolveAccess(roles []RoleRecord, permissions []taskconfig.Permission) (bool, []string) {
	roleSet := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		roleSet[r.Name] = struct{}{}
	}

	isAccessible := false
	seen := make(map[string]struct{})
	var actions []string
	for _, p := range permissions {
		if _, ok := roleSet[p.Role]; !ok {
			continue
		}
		isAccessible = true
		for _, a := range p.Actions {
			if _, exists := seen[a]; !exists {
				seen[a] = struct{}{}
				actions = append(actions, a)
			}
		}
	}
	return isAccessible, actions
}

// hasAction reports whether action exists in the provided actions slice.
func hasAction(actions []string, action string) bool {
	for _, a := range actions {
		if a == action {
			return true
		}
	}
	return false
}
