package user

import (
	"errors"
	"fmt"
	"time"

	"github.com/OpenNSW/nsw-agency/backend/internal/config"
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
	SSOID     string    `gorm:"type:text;uniqueIndex;not null"`
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

func NewUserStore(cfg config.Config) (*UserStore, error) {
	connector, err := database.NewConnector(cfg.DB)
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

	return &UserStore{db: db, agency: cfg.Agency}, nil
}

// FindOrProvision looks up a user by SSOID and creates them if they don't exist.
// ouHandle is the agency identifier from the JWT and must match this instance's
// configured agency before a new user record is created.
// If the user already exists, email and name are synced from the token claims.
func (s *UserStore) FindOrProvision(ssoid, email, name, ouHandle string) (*UserRecord, error) {
	var user UserRecord
	err := s.db.First(&user, "ssoid = ?", ssoid).Error
	if err == nil {
		if user.Email != email || user.Name != name {
			if err := s.db.Model(&user).Updates(map[string]any{
				"email": email,
				"name":  name,
			}).Error; err != nil {
				return nil, fmt.Errorf("failed to sync user attributes: %w", err)
			}
			user.Email = email
			user.Name = name
		}
		return &user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// New user — validate agency before provisioning.
	if ouHandle != s.agency {
		return nil, ErrUnauthorizedAgency
	}

	user = UserRecord{
		SSOID: ssoid,
		Email: email,
		Name:  name,
	}
	if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to provision user: %w", err)
	}
	// Re-fetch in case a concurrent request inserted the same SSOID first.
	if user.UserID == "" {
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
