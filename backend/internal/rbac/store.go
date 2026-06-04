package rbac

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrRoleNotFound = errors.New("role not found")

// RoleRecord represents a role in the database.
type RoleRecord struct {
	ID        string    `gorm:"type:text;primaryKey"`
	Name      string    `gorm:"type:text;not null;uniqueIndex"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (RoleRecord) TableName() string { return "roles" }

func (r *RoleRecord) BeforeCreate(_ *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// UserRoleRecord represents a user-to-role assignment in the database.
type UserRoleRecord struct {
	ID         string    `gorm:"type:text;primaryKey"`
	UserID     string    `gorm:"type:text;not null;uniqueIndex:idx_user_role"`
	RoleID     string    `gorm:"type:text;not null;uniqueIndex:idx_user_role"`
	AssignedAt time.Time `gorm:"autoCreateTime"`
}

func (UserRoleRecord) TableName() string { return "user_roles" }

func (ur *UserRoleRecord) BeforeCreate(_ *gorm.DB) error {
	if ur.ID == "" {
		ur.ID = uuid.New().String()
	}
	return nil
}

// RoleStore handles CRUD operations on roles.
type RoleStore struct {
	db *gorm.DB
}

func NewRoleStore(db *gorm.DB) *RoleStore {
	return &RoleStore{db: db}
}

func (s *RoleStore) Create(name string) (*RoleRecord, error) {
	role := RoleRecord{Name: name}
	if err := s.db.Create(&role).Error; err != nil {
		return nil, fmt.Errorf("failed to create role: %w", err)
	}
	return &role, nil
}

func (s *RoleStore) FindByName(name string) (*RoleRecord, error) {
	var role RoleRecord
	if err := s.db.First(&role, "name = ?", name).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to find role: %w", err)
	}
	return &role, nil
}

func (s *RoleStore) List() ([]RoleRecord, error) {
	var roles []RoleRecord
	if err := s.db.Order("name").Find(&roles).Error; err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}
	return roles, nil
}

// UserRoleStore handles user-to-role assignment operations.
type UserRoleStore struct {
	db *gorm.DB
}

func NewUserRoleStore(db *gorm.DB) *UserRoleStore {
	return &UserRoleStore{db: db}
}

func (s *UserRoleStore) Assign(userID, roleID string) error {
	ur := UserRoleRecord{UserID: userID, RoleID: roleID}
	if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&ur).Error; err != nil {
		return fmt.Errorf("failed to assign role to user: %w", err)
	}
	return nil
}

// GetRolesForUser returns all roles assigned to the given user.
func (s *UserRoleStore) GetRolesForUser(userID string) ([]RoleRecord, error) {
	var roles []RoleRecord
	err := s.db.
		Select("roles.*").
		Joins("INNER JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Find(&roles).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get roles for user: %w", err)
	}
	return roles, nil
}
