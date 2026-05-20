package application

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/OpenNSW/nsw-agency/backend/internal/config"
	"github.com/OpenNSW/nsw-agency/backend/internal/database"
	"github.com/OpenNSW/nsw-agency/backend/internal/feedback"
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
	TaskID             string           `gorm:"type:text;primaryKey"`
	TaskCode           string           `gorm:"type:varchar(100);not null"`
	ConsignmentID         string           `gorm:"type:text;index;not null"`
	ServiceURL         string           `gorm:"type:varchar(512);not null"`                  // URL to send response back to
	Data               JSONB            `gorm:"type:text"`                                   // Injected data from service
	ReviewerResponse   JSONB            `gorm:"type:text"`                                   // Response from reviewer
	Status             string           `gorm:"type:varchar(50);not null;default:'PENDING'"` // PENDING, FEEDBACK_REQUESTED, DONE
	OGAFeedbackHistory []feedback.Entry `gorm:"type:text;serializer:json"`
	ReviewedAt         *time.Time       // When it was reviewed
	CreatedAt          time.Time        `gorm:"autoCreateTime"`
	UpdatedAt          time.Time        `gorm:"autoUpdateTime"`
}

// TableName returns the table name for ApplicationRecord
func (ApplicationRecord) TableName() string {
	return "applications"
}

// ApplicationStore handles database operations for OGA applications
type ApplicationStore struct {
	db *gorm.DB
}

// NewApplicationStore creates a new ApplicationStore with configured database
func NewApplicationStore(cfg config.Config) (*ApplicationStore, error) {
	connector, err := database.NewConnector(cfg.DB)
	if err != nil {
		return nil, err
	}

	db, err := connector.Open()
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
func (s *ApplicationStore) GetByTaskID(taskID string) (*ApplicationRecord, error) {
	var app ApplicationRecord
	if err := s.db.First(&app, "task_id = ?", taskID).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// List retrieves applications with optional status, consignment, and search filters and pagination.
func (s *ApplicationStore) List(ctx context.Context, status string, consignmentID string, search string, offset, limit int) ([]ApplicationRecord, int64, error) {
	var apps []ApplicationRecord
	var total int64

	query := s.db.WithContext(ctx).Model(&ApplicationRecord{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if consignmentID != "" {
		query = query.Where("consignment_id = ?", consignmentID)
	}
	if search != "" {
		query = query.Where("task_id LIKE ? OR consignment_id LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&apps).Error; err != nil {
		return nil, 0, err
	}

	return apps, total, nil
}

// ConsignmentSummary represents a unique consignment with its most recent activity.
type ConsignmentSummary struct {
	ConsignmentID string    `json:"consignmentId"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Status     string    `json:"status"`    // Status of the most recent application
	TaskCount  int       `json:"taskCount"` // Total number of applications in this consignment
}

// ListConsignments returns a paginated list of unique consignment IDs with their latest status, update time, and task count, with optional search.
func (s *ApplicationStore) ListConsignments(ctx context.Context, search string, offset, limit int) ([]ConsignmentSummary, int64, error) {
	var summaries []ConsignmentSummary
	var total int64

	countQuery := s.db.WithContext(ctx).Model(&ApplicationRecord{})
	if search != "" {
		countQuery = countQuery.Where("consignment_id LIKE ?", "%"+search+"%")
	}

	// Count unique consignments
	if err := countQuery.Distinct("consignment_id").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Subquery to get the latest updated_at and the count for each consignment_id
	latestSubquery := s.db.Model(&ApplicationRecord{}).
		Select("consignment_id, MAX(updated_at) as max_updated, COUNT(*) as task_count").
		Group("consignment_id")

	if search != "" {
		latestSubquery = latestSubquery.Where("consignment_id LIKE ?", "%"+search+"%")
	}

	// Join with original table to get the status of the record with that max_updated
	err := s.db.WithContext(ctx).Model(&ApplicationRecord{}).
		Select("applications.consignment_id, applications.updated_at, applications.status, latest.task_count").
		Joins("JOIN (?) as latest ON applications.consignment_id = latest.consignment_id AND applications.updated_at = latest.max_updated", latestSubquery).
		Order("applications.updated_at DESC").
		Offset(offset).
		Limit(limit).
		Scan(&summaries).Error

	if err != nil {
		return nil, 0, err
	}

	return summaries, total, nil
}

func (s *ApplicationStore) UpdateStatus(taskID string, status string, reviewerResponse map[string]any) error {
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

// AppendFeedback appends a feedback entry to the application's history and sets
// the status to FEEDBACK_REQUESTED.
func (s *ApplicationStore) AppendFeedback(taskID string, entry feedback.Entry) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var app ApplicationRecord
		if err := tx.First(&app, "task_id = ?", taskID).Error; err != nil {
			return err
		}
		updated := append(app.OGAFeedbackHistory, entry)
		updatedJSON, err := json.Marshal(updated)
		if err != nil {
			return fmt.Errorf("failed to marshal feedback history: %w", err)
		}
		return tx.Model(&ApplicationRecord{}).
			Where("task_id = ?", taskID).
			Updates(map[string]any{
				"oga_feedback_history": string(updatedJSON),
				"status":               "FEEDBACK_REQUESTED",
				"updated_at":           time.Now(),
			}).Error
	})
}

// UpdateDataAndResetStatus updates the submitted data and resets status to PENDING.
// Called when a trader resubmits after receiving feedback.
func (s *ApplicationStore) UpdateDataAndResetStatus(taskID string, data map[string]any) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}
	return s.db.Model(&ApplicationRecord{}).
		Where("task_id = ?", taskID).
		Updates(map[string]any{
			"data":       string(dataJSON),
			"status":     "PENDING",
			"updated_at": time.Now(),
		}).Error
}

// Delete removes an application by task ID
func (s *ApplicationStore) Delete(taskID string) error {
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
