package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrApplicationNotFound is returned when an application is not found
var ErrApplicationNotFound = errors.New("application not found")

// OGAService handles OGA portal operations
type OGAService interface {
	// CreateApplication creates a new application from injected data
	CreateApplication(ctx context.Context, req *InjectRequest) error

	// GetApplications returns a paginated list of applications (optionally filtered by status)
	GetApplications(ctx context.Context, status string, page, pageSize int) (*PagedResponse[Application], error)

	// GetApplication returns a specific application by task ID
	GetApplication(ctx context.Context, taskID uuid.UUID) (*Application, error)

	// ReviewApplication approves or rejects an application and sends response back to service
	ReviewApplication(ctx context.Context, taskID uuid.UUID, reviewerData map[string]any) error

	// Close closes the service and releases resources
	Close() error
}

type Meta struct {
	VerificationType string `json:"type"`
	VerificationId   string `json:"verificationId"`
}

// InjectRequest represents the incoming data from services
type InjectRequest struct {
	TaskID     uuid.UUID      `json:"taskId"`
	WorkflowID uuid.UUID      `json:"workflowId"`
	Data       map[string]any `json:"data"`
	ServiceURL string         `json:"serviceUrl"` // URL to send response back to
	Meta       *Meta          `json:"meta,omitempty"`
}

// Application represents an application for display in the UI
type Application struct {
	TaskID     uuid.UUID       `json:"taskId"`
	WorkflowID uuid.UUID       `json:"workflowId"`
	ServiceURL string          `json:"serviceUrl"`
	Data       map[string]any  `json:"data"`
	Meta       *Meta           `json:"meta,omitempty"`
	Form       json.RawMessage `json:"form,omitempty"`
	Status     string          `json:"status"`
	ReviewedAt *time.Time      `json:"reviewedAt,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
}

// PagedResponse is a generic paginated response wrapper.
type PagedResponse[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// TaskResponse represents the response sent back to the service
type TaskResponse struct {
	TaskID     uuid.UUID `json:"task_id"`
	WorkflowID uuid.UUID `json:"workflow_id"`
	Payload    any       `json:"payload"`
}

// metaToJSONB converts a *Meta struct to JSONB via JSON round-trip.
func metaToJSONB(m *Meta) (JSONB, error) {
	if m == nil {
		return nil, nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Meta: %w", err)
	}
	var j JSONB
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Meta to JSONB: %w", err)
	}
	return j, nil
}

// metaFromJSONB converts a JSONB map back to a *Meta struct via JSON round-trip.
func metaFromJSONB(j JSONB) *Meta {
	if j == nil {
		return nil
	}
	data, err := json.Marshal(j)
	if err != nil {
		return nil
	}
	var m Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return &m
}

type ogaService struct {
	store      *ApplicationStore
	formStore  *FormStore
	httpClient *http.Client
}

// NewOGAService creates a new OGA service instance with database storage
func NewOGAService(store *ApplicationStore, formStore *FormStore) OGAService {
	return &ogaService{
		store:     store,
		formStore: formStore,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateApplication creates a new application from injected data
func (s *ogaService) CreateApplication(ctx context.Context, req *InjectRequest) error {
	// Validate required fields
	if req.TaskID == uuid.Nil {
		return fmt.Errorf("taskId is required")
	}
	if req.WorkflowID == uuid.Nil {
		return fmt.Errorf("workflowId is required")
	}
	if req.ServiceURL == "" {
		return fmt.Errorf("serviceUrl is required")
	}

	metaJSON, err := metaToJSONB(req.Meta)
	if err != nil {
		return fmt.Errorf("failed to convert meta: %w", err)
	}

	appRecord := &ApplicationRecord{
		TaskID:     req.TaskID,
		WorkflowID: req.WorkflowID,
		ServiceURL: req.ServiceURL,
		Data:       req.Data,
		Meta:       metaJSON,
		Status:     "PENDING",
	}

	if err := s.store.CreateOrUpdate(appRecord); err != nil {
		return fmt.Errorf("failed to store application: %w", err)
	}

	slog.InfoContext(ctx, "application created",
		"taskID", req.TaskID,
		"workflowID", req.WorkflowID)

	return nil
}

// GetApplications returns a paginated list of applications (optionally filtered by status)
func (s *ogaService) GetApplications(ctx context.Context, status string, page, pageSize int) (*PagedResponse[Application], error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	records, total, err := s.store.List(ctx, status, offset, pageSize)
	if err != nil {
		return nil, err
	}

	applications := make([]Application, len(records))
	for i, record := range records {
		meta := metaFromJSONB(record.Meta)
		applications[i] = Application{
			TaskID:     record.TaskID,
			WorkflowID: record.WorkflowID,
			ServiceURL: record.ServiceURL,
			Data:       record.Data,
			Meta:       meta,
			Status:     record.Status,
			ReviewedAt: record.ReviewedAt,
			CreatedAt:  record.CreatedAt,
			UpdatedAt:  record.UpdatedAt,
		}
	}

	return &PagedResponse[Application]{
		Items:    applications,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetApplication returns a specific application by task ID
func (s *ogaService) GetApplication(ctx context.Context, taskID uuid.UUID) (*Application, error) {
	record, err := s.store.GetByTaskID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApplicationNotFound
		}
		return nil, fmt.Errorf("failed to get application: %w", err)
	}

	meta := metaFromJSONB(record.Meta)
	app := &Application{
		TaskID:     record.TaskID,
		WorkflowID: record.WorkflowID,
		ServiceURL: record.ServiceURL,
		Data:       record.Data,
		Meta:       meta,
		Status:     record.Status,
		ReviewedAt: record.ReviewedAt,
		CreatedAt:  record.CreatedAt,
		UpdatedAt:  record.UpdatedAt,
	}

	// Attach form: look up by meta, fall back to default
	formID := FormIDFromMeta(meta)
	if formID != "" {
		if form, err := s.formStore.GetForm(formID); err == nil {
			app.Form = form
		} else {
			slog.WarnContext(ctx, "form not found for application, using default", "taskID", taskID, "formID", formID)
			if form, err := s.formStore.GetDefaultForm(); err == nil {
				app.Form = form
			}
		}
	} else {
		if form, err := s.formStore.GetDefaultForm(); err == nil {
			app.Form = form
		}
	}

	return app, nil
}

// ReviewApplication approves or rejects an application and sends response back to service
func (s *ogaService) ReviewApplication(ctx context.Context, taskID uuid.UUID, reviewerResponse map[string]any) error {
	// Get the application to retrieve service URL and workflow ID
	app, err := s.GetApplication(ctx, taskID)
	if err != nil {
		return err
	}

	decision, ok := reviewerResponse["decision"].(string)
	if !ok || decision == "" {
		return fmt.Errorf("reviewerResponse must contain a non-empty 'decision' string")
	}
	status := decision

	if err := s.store.UpdateStatus(taskID, status, reviewerResponse); err != nil {
		return fmt.Errorf("failed to update application status: %w", err)
	}

	// Prepare response payload for the service
	response := TaskResponse{
		TaskID:     app.TaskID,
		WorkflowID: app.WorkflowID,
		Payload: map[string]any{
			"action":  "OGA_VERIFICATION",
			"content": reviewerResponse,
		},
	}

	// Send response back to the service
	if err := s.sendToService(ctx, app.ServiceURL, response); err != nil {
		slog.ErrorContext(ctx, "failed to send response to service",
			"taskID", taskID,
			"serviceURL", app.ServiceURL,
			"error", err)
		return fmt.Errorf("failed to send response to service: %w", err)
	}

	slog.InfoContext(ctx, "application reviewed and response sent",
		"taskID", taskID,
		"serviceURL", app.ServiceURL)

	return nil
}

// sendToService sends the task response to the originating service
func (s *ogaService) sendToService(ctx context.Context, serviceURL string, response TaskResponse) error {
	jsonData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serviceURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("service returned status code %d", resp.StatusCode)
	}

	return nil
}

// Close closes the service and releases resources
func (s *ogaService) Close() error {
	if s.store != nil {
		return s.store.Close()
	}
	return nil
}
