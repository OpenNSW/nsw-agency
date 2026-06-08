package rbac

import (
	"gorm.io/gorm"
)

// RoleService handles business logic related to roles and user-role assignments.
type RoleService struct {
	roleStore     *RoleStore
	userRoleStore *UserRoleStore
}

// NewRoleService creates a new RoleService instance.
func NewRoleService(db *gorm.DB) *RoleService {
	return &RoleService{
		roleStore:     NewRoleStore(db),
		userRoleStore: NewUserRoleStore(db),
	}
}

// Create creates a new role.
func (s *RoleService) Create(name string) (*RoleRecord, error) {
	return s.roleStore.Create(name)
}

// FindByName finds a role by its name.
func (s *RoleService) FindByName(name string) (*RoleRecord, error) {
	return s.roleStore.FindByName(name)
}

// List lists all roles in alphabetical order.
func (s *RoleService) List() ([]RoleRecord, error) {
	return s.roleStore.List()
}

// Assign assigns a role to a user.
func (s *RoleService) Assign(userID, roleID string) error {
	return s.userRoleStore.Assign(userID, roleID)
}

// GetRolesForUser returns all roles assigned to the given user.
func (s *RoleService) GetRolesForUser(userID string) ([]RoleRecord, error) {
	return s.userRoleStore.GetRolesForUser(userID)
}
