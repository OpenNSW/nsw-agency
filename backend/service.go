package oga

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/OpenNSW/nsw/internal/workflow/model"
	"github.com/google/uuid"
)

// ErrApplicationNotFound is returned when an application is not found
var ErrApplicationNotFound = errors.New("application not found")

// OGAService handles OGA portal operations
// For MVP, this service does not persist to database - it only manages in-memory state
type OGAService interface {
	// AddApplication adds an application ready for review
	AddApplication(ctx context.Context, notification model.OGATaskNotification) error

	// GetApplications returns all applications ready for review
	GetApplications(ctx context.Context) ([]Application, error)

	// GetApplication returns a specific application by task ID
	GetApplication(ctx context.Context, taskID uuid.UUID) (*Application, error)

	// RemoveApplication removes an application from the ready list (after approval/rejection)
	RemoveApplication(ctx context.Context, taskID uuid.UUID) error
}

// Application represents an application ready for OGA review
type Application struct {
	TaskID        uuid.UUID `json:"taskId"`
	ConsignmentID uuid.UUID `json:"consignmentId"`
	FormID        string    `json:"formId"`
	Status        string    `json:"status"`
}

type ogaService struct {
	applications map[uuid.UUID]Application // In-memory store of applications ready for review
	mu           sync.RWMutex              // Mutex for thread-safe access
}

// NewOGAService creates a new OGA service instance
func NewOGAService() OGAService {
	return &ogaService{
		applications: make(map[uuid.UUID]Application),
	}
}

// AddApplication adds an application ready for review
func (s *ogaService) AddApplication(ctx context.Context, notification model.OGATaskNotification) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	application := Application{
		TaskID:        notification.TaskID,
		ConsignmentID: notification.ConsignmentID,
		FormID:        notification.FormID,
		Status:        notification.Status,
	}

	s.applications[notification.TaskID] = application

	slog.InfoContext(ctx, "application added for OGA review",
		"taskID", notification.TaskID,
		"consignmentID", notification.ConsignmentID,
		"formID", notification.FormID)

	return nil
}

// GetApplications returns all applications ready for review
func (s *ogaService) GetApplications(ctx context.Context) ([]Application, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	applications := make([]Application, 0, len(s.applications))
	for _, app := range s.applications {
		applications = append(applications, app)
	}

	return applications, nil
}

// GetApplication returns a specific application by task ID
func (s *ogaService) GetApplication(ctx context.Context, taskID uuid.UUID) (*Application, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, exists := s.applications[taskID]
	if !exists {
		return nil, ErrApplicationNotFound
	}

	return &app, nil
}

// RemoveApplication removes an application from the ready list
func (s *ogaService) RemoveApplication(ctx context.Context, taskID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.applications, taskID)

	slog.InfoContext(ctx, "application removed from OGA review list",
		"taskID", taskID)

	return nil
}
