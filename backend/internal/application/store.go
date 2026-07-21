package application

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/OpenNSW/nsw-agency/backend/internal/database"
	"github.com/OpenNSW/nsw-agency/backend/internal/feedback"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// ConsignmentRecord represents a consignment (workflow) in the Agency database.
// Each consignment groups one or more ApplicationRecords.
type ConsignmentRecord struct {
	ID        string    `gorm:"type:text;primaryKey"`
	Status    string    `gorm:"type:varchar(50);not null;default:'PENDING'"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName returns the table name for ConsignmentRecord
func (ConsignmentRecord) TableName() string {
	return "consignments"
}

// ApplicationRecord represents an application (task) in the Agency database
type ApplicationRecord struct {
	TaskID                string            `gorm:"type:text;primaryKey"`
	TaskCode              string            `gorm:"type:varchar(100);not null"`
	ConsignmentID         string            `gorm:"type:text;index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Consignment           ConsignmentRecord `gorm:"foreignKey:ConsignmentID;references:ID"`
	ServiceURL            string            `gorm:"type:varchar(512);not null"`                  // URL to send response back to
	Data                  JSONB             `gorm:"type:text"`                                   // Injected data from service
	ReviewerResponse      JSONB             `gorm:"type:text"`                                   // Response from reviewer
	Status                string            `gorm:"type:varchar(50);not null;default:'PENDING'"` // PENDING, FEEDBACK_REQUESTED, DONE
	AgencyFeedbackHistory []feedback.Entry  `gorm:"type:text;serializer:json"`
	ReviewedAt            *time.Time        // When it was reviewed
	CreatedAt             time.Time         `gorm:"autoCreateTime"`
	UpdatedAt             time.Time         `gorm:"autoUpdateTime"`
}

// TableName returns the table name for ApplicationRecord
func (ApplicationRecord) TableName() string {
	return "applications"
}

// UploadedFile tracks files uploaded by the agency before they are associated with an application.
type UploadedFile struct {
	Key        string    `gorm:"primaryKey;type:varchar(255)"`
	UploadedBy string    `gorm:"type:varchar(255);not null"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
}

// TableName returns the table name for UploadedFile
func (UploadedFile) TableName() string {
	return "uploaded_files"
}

// ApplicationFile maps an application to its associated file keys for indexing.
type ApplicationFile struct {
	ApplicationID string `gorm:"primaryKey;type:varchar(255)"`
	FileKey       string `gorm:"primaryKey;type:varchar(255);index"`
}

// TableName returns the table name for ApplicationFile
func (ApplicationFile) TableName() string {
	return "application_files"
}

// ApplicationStore handles database operations for Agency applications
type ApplicationStore struct {
	db *gorm.DB
}

// NewApplicationStore creates a new ApplicationStore with configured database.
// Schema must be applied before starting the server via the migrate command.
func NewApplicationStore(cfg database.Config) (*ApplicationStore, error) {
	connector, err := database.NewConnector(cfg)
	if err != nil {
		return nil, err
	}

	db, err := connector.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate tracking tables
	if err := db.AutoMigrate(&UploadedFile{}, &ApplicationFile{}); err != nil {
		return nil, fmt.Errorf("failed to auto-migrate tracking tables: %w", err)
	}

	return &ApplicationStore{db: db}, nil
}

// CreateOrUpdate creates or updates an application record and its parent consignment.
func (s *ApplicationStore) CreateOrUpdate(app *ApplicationRecord) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Upsert the consignment first so the FK reference exists.
		consignment := ConsignmentRecord{ID: app.ConsignmentID, Status: app.Status}
		if err := tx.Save(&consignment).Error; err != nil {
			return fmt.Errorf("failed to upsert consignment: %w", err)
		}
		if err := tx.Save(app).Error; err != nil {
			return err
		}
		return s.syncApplicationFiles(tx, app.TaskID)
	})
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

	if err := query.Order("CASE WHEN status = 'PENDING' THEN 0 WHEN status = 'FEEDBACK_REQUESTED' THEN 1 ELSE 2 END ASC, created_at DESC").Offset(offset).Limit(limit).Find(&apps).Error; err != nil {
		return nil, 0, err
	}

	return apps, total, nil
}

// ConsignmentSummary represents a unique consignment with its most recent activity.
type ConsignmentSummary struct {
	ConsignmentID string    `json:"consignmentId"`
	UpdatedAt     time.Time `json:"updatedAt"`
	Status        string    `json:"status"`    // Status of the most recent application
	TaskCount     int       `json:"taskCount"` // Total number of applications in this consignment
}

// ListConsignments returns a paginated list of consignments with task count and optional search.
func (s *ApplicationStore) ListConsignments(ctx context.Context, search string, offset, limit int) ([]ConsignmentSummary, int64, error) {
	var summaries []ConsignmentSummary
	var total int64

	countQ := s.db.WithContext(ctx).Model(&ConsignmentRecord{})
	if search != "" {
		countQ = countQ.Where("id LIKE ?", "%"+search+"%")
	}
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	dataQ := s.db.WithContext(ctx).Model(&ConsignmentRecord{}).
		Select("consignments.id AS consignment_id, consignments.status, consignments.updated_at, COUNT(applications.task_id) AS task_count").
		Joins("LEFT JOIN applications ON applications.consignment_id = consignments.id").
		Group("consignments.id, consignments.status, consignments.updated_at").
		Order("consignments.updated_at DESC").
		Offset(offset).
		Limit(limit)

	if search != "" {
		dataQ = dataQ.Where("consignments.id LIKE ?", "%"+search+"%")
	}

	if err := dataQ.Scan(&summaries).Error; err != nil {
		return nil, 0, err
	}

	return summaries, total, nil
}

// UpdateStatus updates the status of an application and propagates it to the parent consignment.
func (s *ApplicationStore) UpdateStatus(taskID string, status string, reviewerResponse map[string]any) error {
	now := time.Now()

	jsonResponse, err := json.Marshal(reviewerResponse)
	if err != nil {
		return fmt.Errorf("failed to marshal reviewer response: %w", err)
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ApplicationRecord{}).
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

		var app ApplicationRecord
		if err := tx.Select("consignment_id").Where("task_id = ?", taskID).First(&app).Error; err != nil {
			return fmt.Errorf("failed to fetch consignment_id: %w", err)
		}

		if err := tx.Model(&ConsignmentRecord{}).
			Where("id = ?", app.ConsignmentID).
			Updates(map[string]any{
				"status":     status,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		return s.syncApplicationFiles(tx, taskID)
	})
}

// AppendFeedback appends a feedback entry to the application's history and sets
// the status to FEEDBACK_REQUESTED.
func (s *ApplicationStore) AppendFeedback(taskID string, entry feedback.Entry) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var app ApplicationRecord
		if err := tx.First(&app, "task_id = ?", taskID).Error; err != nil {
			return err
		}
		updated := append(app.AgencyFeedbackHistory, entry)
		updatedJSON, err := json.Marshal(updated)
		if err != nil {
			return fmt.Errorf("failed to marshal feedback history: %w", err)
		}

		now := time.Now()

		if err := tx.Model(&ApplicationRecord{}).
			Where("task_id = ?", taskID).
			Updates(map[string]any{
				"agency_feedback_history": string(updatedJSON),
				"status":                  "FEEDBACK_REQUESTED",
				"updated_at":              now,
			}).Error; err != nil {
			return err
		}

		if err := tx.Model(&ConsignmentRecord{}).
			Where("id = ?", app.ConsignmentID).
			Updates(map[string]any{
				"status":     "FEEDBACK_REQUESTED",
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		return s.syncApplicationFiles(tx, taskID)
	})
}

// UpdateDataAndResetStatus updates the submitted data and resets status to PENDING.
// Called when a trader resubmits after receiving feedback.
func (s *ApplicationStore) UpdateDataAndResetStatus(taskID string, data map[string]any) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var app ApplicationRecord
		if err := tx.Select("consignment_id").Where("task_id = ?", taskID).First(&app).Error; err != nil {
			return err
		}

		now := time.Now()

		if err := tx.Model(&ApplicationRecord{}).
			Where("task_id = ?", taskID).
			Updates(map[string]any{
				"data":       string(dataJSON),
				"status":     "PENDING",
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		if err := tx.Model(&ConsignmentRecord{}).
			Where("id = ?", app.ConsignmentID).
			Updates(map[string]any{
				"status":     "PENDING",
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		return s.syncApplicationFiles(tx, taskID)
	})
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

// DB returns the underlying gorm.DB connection.
func (s *ApplicationStore) DB() *gorm.DB {
	return s.db
}

// GetTaskCode implements rbac.TaskCodeResolver.
func (s *ApplicationStore) GetTaskCode(ctx context.Context, taskID string) (string, error) {
	var app ApplicationRecord
	if err := s.db.WithContext(ctx).Select("task_code").First(&app, "task_id = ?", taskID).Error; err != nil {
		return "", err
	}
	return app.TaskCode, nil
}

// extractFileKeys recursively finds all string values in a nested JSON structure (map, slice, etc.)
func extractFileKeys(val any) []string {
	var keys []string
	switch v := val.(type) {
	case string:
		if len(v) > 0 {
			keys = append(keys, v)
		}
	case map[string]any:
		for _, item := range v {
			keys = append(keys, extractFileKeys(item)...)
		}
	case []any:
		for _, item := range v {
			keys = append(keys, extractFileKeys(item)...)
		}
	}
	return keys
}

func extractKeysFromJSONB(j JSONB) []string {
	if len(j) == 0 {
		return nil
	}
	bytes, err := json.Marshal(j)
	if err != nil {
		return nil
	}
	var val any
	if json.Unmarshal(bytes, &val) != nil {
		return nil
	}
	return extractFileKeys(val)
}

// syncApplicationFiles extracts file keys from JSON fields of the application and syncs them to application_files
func (s *ApplicationStore) syncApplicationFiles(tx *gorm.DB, taskID string) error {
	var app ApplicationRecord
	if err := tx.First(&app, "task_id = ?", taskID).Error; err != nil {
		return fmt.Errorf("failed to fetch application for sync: %w", err)
	}

	uniqueKeys := make(map[string]struct{})

	// Extract from Data (JSONB)
	for _, key := range extractKeysFromJSONB(app.Data) {
		uniqueKeys[key] = struct{}{}
	}

	// Extract from ReviewerResponse (JSONB)
	for _, key := range extractKeysFromJSONB(app.ReviewerResponse) {
		uniqueKeys[key] = struct{}{}
	}

	// Extract from AgencyFeedbackHistory
	if len(app.AgencyFeedbackHistory) > 0 {
		historyBytes, err := json.Marshal(app.AgencyFeedbackHistory)
		if err == nil {
			var parsedHistory any
			if json.Unmarshal(historyBytes, &parsedHistory) == nil {
				for _, key := range extractFileKeys(parsedHistory) {
					uniqueKeys[key] = struct{}{}
				}
			}
		}
	}

	// Delete old mappings
	if err := tx.Where("application_id = ?", taskID).Delete(&ApplicationFile{}).Error; err != nil {
		return fmt.Errorf("failed to clear old application files: %w", err)
	}

	// Insert new mappings
	if len(uniqueKeys) > 0 {
		var mappings []ApplicationFile
		for key := range uniqueKeys {
			if len(key) >= 10 {
				mappings = append(mappings, ApplicationFile{
					ApplicationID: taskID,
					FileKey:       key,
				})
			}
		}
		if len(mappings) > 0 {
			if err := tx.Create(&mappings).Error; err != nil {
				return fmt.Errorf("failed to insert application files mapping: %w", err)
			}
		}
	}

	return nil
}

// GetUploadedFile retrieves an UploadedFile record by its key.
func (s *ApplicationStore) GetUploadedFile(ctx context.Context, key string) (*UploadedFile, error) {
	var upload UploadedFile
	if err := s.db.WithContext(ctx).First(&upload, "key = ?", key).Error; err != nil {
		return nil, err
	}
	return &upload, nil
}

// FindApplicationsByFileKey finds all applications referencing the given file key in application_files.
func (s *ApplicationStore) FindApplicationsByFileKey(ctx context.Context, key string) ([]ApplicationRecord, error) {
	var apps []ApplicationRecord
	err := s.db.WithContext(ctx).
		Joins("JOIN application_files ON application_files.application_id = applications.task_id").
		Where("application_files.file_key = ?", key).
		Find(&apps).Error
	if err != nil {
		return nil, err
	}
	return apps, nil
}

// TrackUpload tracks a newly uploaded file key by inserting it into uploaded_files.
func (s *ApplicationStore) TrackUpload(ctx context.Context, key string, uploadedBy string) error {
	upload := UploadedFile{
		Key:        key,
		UploadedBy: uploadedBy,
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&upload).Error
}
