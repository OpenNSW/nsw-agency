package user

import (
	"errors"
	"fmt"
	"time"

	"github.com/OpenNSW/nsw-agency/backend/internal/database"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrUnauthorizedAgency is returned when the user's JWT agency does not match
// the agency this service instance is configured for.
var ErrUnauthorizedAgency = errors.New("user does not belong to this agency")

// ErrUserNotFound is returned when no user with the given email exists in the
// database. Users must be pre-seeded via the seed CLI before they can log in.
var ErrUserNotFound = errors.New("user not found — ensure the user has been seeded")

type UserRecord struct {
	UserID    string    `gorm:"type:text;primaryKey"`
	SSOID     *string   `gorm:"column:ssoid;type:text;uniqueIndex"`
	Email     string    `gorm:"type:text"`
	Name      string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (UserRecord) TableName() string {
	return "users"
}

// BeforeCreate generates a UUID v4 for UserID if one is not already set.
func (u *UserRecord) BeforeCreate(_ *gorm.DB) error {
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

	return &UserStore{db: db, agency: expectedOU}, nil
}

// GetOrCreateUser implements auth.UserProfileService. It finds the pre-seeded
// user by email, syncs their SSOID from the token if not yet set, and returns
// the internal UserID. Returns an error if the user has not been seeded.
func (s *UserStore) GetOrCreateUser(idpUserID, email, givenName, phone, organizationID, ouHandle string) (*string, error) {
	u, err := s.FindAndSync(idpUserID, email, givenName, ouHandle)
	if err != nil {
		return nil, err
	}
	return &u.UserID, nil
}

// FindAndSync looks up a pre-seeded user by email and syncs their SSOID from
// the token on first login. Returns ErrUserNotFound if no matching user exists.
func (s *UserStore) FindAndSync(ssoid, email, name, ouHandle string) (*UserRecord, error) {
	if ouHandle != s.agency {
		return nil, ErrUnauthorizedAgency
	}

	var user UserRecord
	if err := s.db.First(&user, "email = ?", email).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	updates := map[string]any{}

	// Sync SSOID from token on first login (when not yet set).
	if ssoid != "" && (user.SSOID == nil || *user.SSOID == "") {
		updates["ssoid"] = ssoid
	}

	// Sync name if provided and changed.
	if name != "" && user.Name != name {
		updates["name"] = name
	}

	if len(updates) > 0 {
		if err := s.db.Model(&user).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("failed to sync user attributes: %w", err)
		}
		if v, ok := updates["ssoid"].(string); ok {
			user.SSOID = &v
		}
		if v, ok := updates["name"].(string); ok {
			user.Name = v
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
