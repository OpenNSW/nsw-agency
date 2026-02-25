package internal

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// JSONB is a custom type for storing JSON data in SQLite
type JSONB map[string]any

// Value implements the driver.Valuer interface
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface
func (j *JSONB) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to unmarshal JSONB value: %v", value)
	}

	return json.Unmarshal(bytes, j)
}

// ApplicationRecord represents an application in the OGA database
type ApplicationRecord struct {
	TaskID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	WorkflowID       uuid.UUID  `gorm:"type:uuid;index;not null"`
	ServiceURL       string     `gorm:"type:varchar(512);not null"`                  // URL to send response back to
	Data             JSONB      `gorm:"type:text"`                                   // Injected data from service
	Meta             JSONB      `gorm:"type:text"`                                   // Meta Information on Rendering the form
	ReviewerResponse JSONB      `gorm:"type:text"`                                   // Response from reviewer
	Status           string     `gorm:"type:varchar(50);not null;default:'PENDING'"` // PENDING, APPROVED, REJECTED
	ReviewedAt       *time.Time `gorm:"type:datetime"`                               // When it was reviewed
	CreatedAt        time.Time  `gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime"`
}

// TableName returns the table name for ApplicationRecord
func (ApplicationRecord) TableName() string {
	return "applications"
}

// ApplicationStore handles database operations for OGA applications
type ApplicationStore struct {
	db *gorm.DB
}

// NewApplicationStore creates a new ApplicationStore with SQLite database
func NewApplicationStore(dbPath string) (*ApplicationStore, error) {
	if dbPath == "" {
		dbPath = "oga_applications.db"
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&ApplicationRecord{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &ApplicationStore{db: db}, nil
}

// CreateOrUpdate creates or updates an application record
func (s *ApplicationStore) CreateOrUpdate(app *ApplicationRecord) error {
	return s.db.Save(app).Error
}

// GetByTaskID retrieves an application by task ID
func (s *ApplicationStore) GetByTaskID(taskID uuid.UUID) (*ApplicationRecord, error) {
	var app ApplicationRecord
	if err := s.db.First(&app, "task_id = ?", taskID).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// List retrieves applications with optional status filter and pagination.
func (s *ApplicationStore) List(ctx context.Context, status string, offset, limit int) ([]ApplicationRecord, int64, error) {
	var apps []ApplicationRecord
	var total int64

	query := s.db.WithContext(ctx).Model(&ApplicationRecord{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&apps).Error; err != nil {
		return nil, 0, err
	}

	return apps, total, nil
}

func (s *ApplicationStore) UpdateStatus(taskID uuid.UUID, status string, reviewerResponse map[string]any) error {
	now := time.Now()

	// Marshal the map to JSON
	jsonResponse, err := json.Marshal(reviewerResponse)
	if err != nil {
		return fmt.Errorf("failed to marshal reviewer response: %w", err)
	}

	result := s.db.Model(&ApplicationRecord{}).
		Where("task_id = ?", taskID).
		Updates(map[string]any{
			"status":            status,
			"reviewed_at":       now,
			"updated_at":        now,
			"reviewer_response": jsonResponse,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("application with task_id %s not found", taskID)
	}
	return nil
}

// Delete removes an application by task ID
func (s *ApplicationStore) Delete(taskID uuid.UUID) error {
	return s.db.Delete(&ApplicationRecord{}, "task_id = ?", taskID).Error
}

// Close closes the database connection
func (s *ApplicationStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
