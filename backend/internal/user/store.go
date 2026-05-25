package user

import (
	"errors"
	"fmt"
	"time"

	"github.com/OpenNSW/nsw-agency/backend/internal/database"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrUnauthorizedAgency is returned when the user's JWT agency does not match
// the agency this service instance is configured for.
var ErrUnauthorizedAgency = errors.New("user does not belong to this agency")

type UserRecord struct {
	UserID    string    `gorm:"type:text;primaryKey"`
	SSOID     string    `gorm:"column:ssoid;type:text;uniqueIndex;not null"`
	Email     string    `gorm:"type:text"`
	Name      string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (UserRecord) TableName() string {
	return "users"
}

// BeforeCreate generates a UUID v4 for UserID if one is not already set.
func (u *UserRecord) BeforeCreate(tx *gorm.DB) error {
	if u.UserID == "" {
		u.UserID = uuid.New().String()
	}
	return nil
}

type UserStore struct {
	db     *gorm.DB
	agency string
}

func NewUserStore(dbCfg database.Config, expectedOU string) (*UserStore, error) {
	connector, err := database.NewConnector(dbCfg)
	if err != nil {
		return nil, err
	}

	db, err := connector.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.AutoMigrate(&UserRecord{}); err != nil {
		return nil, fmt.Errorf("failed to migrate users table: %w", err)
	}

	return &UserStore{db: db, agency: expectedOU}, nil
}

// GetOrCreateUser implements auth.UserProfileService, enabling JIT provisioning
// via the auth middleware. Returns the internal UserID on success.
func (s *UserStore) GetOrCreateUser(idpUserID, email, givenName, phone, organizationID, ouHandle string) (*string, error) {
	u, err := s.FindOrProvision(idpUserID, email, givenName, ouHandle)
	if err != nil {
		return nil, err
	}
	return &u.UserID, nil
}

// FindOrProvision looks up a user by SSOID and creates them if they don't exist.
// ouHandle is validated against the configured agency for every call.
// If the user already exists, email is synced; name is only synced when non-empty.
func (s *UserStore) FindOrProvision(ssoid, email, name, ouHandle string) (*UserRecord, error) {
	if ouHandle != s.agency {
		return nil, ErrUnauthorizedAgency
	}

	var user UserRecord
	err := s.db.First(&user, "ssoid = ?", ssoid).Error
	if err == nil {
		needsUpdate := user.Email != email || (name != "" && user.Name != name)
		if needsUpdate {
			updates := map[string]any{"email": email}
			if name != "" {
				updates["name"] = name
			}
			if err := s.db.Model(&user).Updates(updates).Error; err != nil {
				return nil, fmt.Errorf("failed to sync user attributes: %w", err)
			}
			user.Email = email
			if name != "" {
				user.Name = name
			}
		}
		return &user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	user = UserRecord{
		SSOID: ssoid,
		Email: email,
		Name:  name,
	}
	result := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&user)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to provision user: %w", result.Error)
	}
	// RowsAffected == 0 means a concurrent request inserted the same SSOID first.
	if result.RowsAffected == 0 {
		if err := s.db.First(&user, "ssoid = ?", ssoid).Error; err != nil {
			return nil, fmt.Errorf("failed to fetch provisioned user: %w", err)
		}
	}
	return &user, nil
}

func (s *UserStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
