package rbac

import (
	"errors"
	"fmt"
	"time"

	"github.com/OpenNSW/nsw-agency/backend/internal/database"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrRoleNotFound = errors.New("role not found")

// RoleRecord represents a role in the database.
type RoleRecord struct {
	ID          string    `gorm:"type:text;primaryKey"`
	Name        string    `gorm:"type:text;not null;uniqueIndex"`
	Description string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
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
	UserID     string    `gorm:"type:text;not null;index"`
	RoleID     string    `gorm:"type:text;not null"`
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

func NewRoleStore(dbCfg database.Config) (*RoleStore, error) {
	connector, err := database.NewConnector(dbCfg)
	if err != nil {
		return nil, err
	}
	db, err := connector.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return &RoleStore{db: db}, nil
}

func (s *RoleStore) Create(name, description string) (*RoleRecord, error) {
	role := RoleRecord{Name: name, Description: description}
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

func (s *RoleStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// UserRoleStore handles user-to-role assignment operations.
type UserRoleStore struct {
	db *gorm.DB
}

func NewUserRoleStore(dbCfg database.Config) (*UserRoleStore, error) {
	connector, err := database.NewConnector(dbCfg)
	if err != nil {
		return nil, err
	}
	db, err := connector.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return &UserRoleStore{db: db}, nil
}

func (s *UserRoleStore) Assign(userID, roleID string) error {
	ur := UserRoleRecord{UserID: userID, RoleID: roleID}
	if err := s.db.Create(&ur).Error; err != nil {
		return fmt.Errorf("failed to assign role to user: %w", err)
	}
	return nil
}

// GetRolesForUser returns all roles assigned to the given user.
func (s *UserRoleStore) GetRolesForUser(userID string) ([]RoleRecord, error) {
	var roles []RoleRecord
	err := s.db.
		Joins("INNER JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Find(&roles).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get roles for user: %w", err)
	}
	return roles, nil
}

func (s *UserRoleStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
