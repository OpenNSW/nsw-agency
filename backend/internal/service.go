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

	"github.com/OpenNSW/nsw/oga/internal/feedback"
	"github.com/OpenNSW/nsw/oga/pkg/httpclient"
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
	GetApplication(ctx context.Context, taskID string) (*Application, error)

	// ReviewApplication approves or rejects an application and sends response back to service
	ReviewApplication(ctx context.Context, taskID string, reviewerData map[string]any) error

	// FeedbackApplication sends a change-request feedback to the trader via the NSW task API
	// and updates the application status to FEEDBACK_REQUESTED.
	FeedbackApplication(ctx context.Context, taskID string, content map[string]any) error

	// GetDownloadURL fetches a download URL for a key from the main backend.
	GetDownloadURL(ctx context.Context, key string) (string, error)

	// Close closes the service and releases resources
	Close() error
}

type Meta struct {
	VerificationType string `json:"type"`
	VerificationId   string `json:"verificationId"`
}

// InjectRequest represents the incoming data from services
type InjectRequest struct {
	TaskID             string           `json:"taskId"`
	WorkflowID         string           `json:"workflowId"`
	Data               map[string]any   `json:"data"`
	ServiceURL         string           `json:"serviceUrl"` // URL to send response back to
	Meta               *Meta            `json:"meta,omitempty"`
	OGAFeedbackHistory []map[string]any `json:"ogafeedbackhistory,omitempty"`
}

// Application represents an application for display in the UI
type Application struct {
	TaskID          string           `json:"taskId"`
	WorkflowID      string           `json:"workflowId"`
	ServiceURL      string           `json:"serviceUrl"`
	Data            map[string]any   `json:"data"`
	Meta            *Meta            `json:"meta,omitempty"`
	Form            json.RawMessage  `json:"form,omitempty"`
	OgaForm         json.RawMessage  `json:"ogaForm,omitempty"`
	Status          string           `json:"status"`
	FeedbackHistory []feedback.Entry `json:"feedbackHistory,omitempty"`
	ReviewedAt      *time.Time       `json:"reviewedAt,omitempty"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
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
	TaskID     string `json:"task_id"`
	WorkflowID string `json:"workflow_id"`
	Payload    any    `json:"payload"`
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
	httpClient *httpclient.Client
}

// NewOGAService creates a new OGA service instance with database storage
func NewOGAService(store *ApplicationStore, formStore *FormStore, httpClient *httpclient.Client) OGAService {
	return &ogaService{
		store:      store,
		formStore:  formStore,
		httpClient: httpClient,
	}
}

// CreateApplication creates a new application from injected data.
// When a trader resubmits after receiving feedback (FEEDBACK_REQUESTED), it updates
// the submitted data and resets the status to PENDING for re-review.
func (s *ogaService) CreateApplication(ctx context.Context, req *InjectRequest) error {
	// Validate required fields
	if req.TaskID == "" {
		return fmt.Errorf("taskId is required")
	}
	if req.WorkflowID == "" {
		return fmt.Errorf("workflowId is required")
	}
	if req.ServiceURL == "" {
		return fmt.Errorf("serviceUrl is required")
	}

	// Re-submission after feedback: preserve history, only update data and reset status.
	existing, err := s.store.GetByTaskID(req.TaskID)
	if err == nil && existing.Status == "FEEDBACK_REQUESTED" {
		slog.InfoContext(ctx, "trader resubmitted after feedback, resetting to PENDING",
			"taskID", req.TaskID)
		return s.store.UpdateDataAndResetStatus(req.TaskID, req.Data)
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
func (s *ogaService) GetApplication(ctx context.Context, taskID string) (*Application, error) {
	record, err := s.store.GetByTaskID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApplicationNotFound
		}
		return nil, fmt.Errorf("failed to get application: %w", err)
	}

	meta := metaFromJSONB(record.Meta)
	app := &Application{
		TaskID:          record.TaskID,
		WorkflowID:      record.WorkflowID,
		ServiceURL:      record.ServiceURL,
		Data:            record.Data,
		Meta:            meta,
		Status:          record.Status,
		FeedbackHistory: feedbackHistoryFromRaw(record.OGAFeedbackHistory),
		ReviewedAt:      record.ReviewedAt,
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
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
		// Try to load an oga form (view template)
		ogaFormID := formID + ".view"
		if ogaForm, err := s.formStore.GetForm(ogaFormID); err == nil {
			app.OgaForm = ogaForm
		}
	} else {
		if form, err := s.formStore.GetDefaultForm(); err == nil {
			app.Form = form
		}
	}

	return app, nil
}

// ReviewApplication approves or rejects an application and sends response back to service
func (s *ogaService) ReviewApplication(ctx context.Context, taskID string, reviewerResponse map[string]any) error {
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

// FeedbackApplication sends OGA feedback to the trader via the NSW task API
// and appends the entry to the application's feedback history.
func (s *ogaService) FeedbackApplication(ctx context.Context, taskID string, content map[string]any) error {
	app, err := s.GetApplication(ctx, taskID)
	if err != nil {
		return err
	}

	entry := feedback.Entry{
		Content:   content,
		Timestamp: time.Now().UTC(),
		Round:     len(app.FeedbackHistory) + 1,
	}

	entryRaw, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal feedback entry: %w", err)
	}
	var entryMap map[string]any
	if err := json.Unmarshal(entryRaw, &entryMap); err != nil {
		return fmt.Errorf("failed to convert feedback entry: %w", err)
	}

	if err := s.store.AppendFeedback(taskID, entryMap); err != nil {
		return fmt.Errorf("failed to store feedback: %w", err)
	}

	response := TaskResponse{
		TaskID:     app.TaskID,
		WorkflowID: app.WorkflowID,
		Payload: map[string]any{
			"action":  "OGA_VERIFICATION_FEEDBACK",
			"content": content,
		},
	}

	if err := s.sendToService(ctx, app.ServiceURL, response); err != nil {
		slog.ErrorContext(ctx, "failed to send feedback to NSW service",
			"taskID", taskID, "serviceURL", app.ServiceURL, "error", err)
		return fmt.Errorf("failed to send feedback to service: %w", err)
	}

	slog.InfoContext(ctx, "feedback sent", "taskID", taskID, "round", entry.Round)
	return nil
}

// GetDownloadURL returns a download URL for a file stored in the main backend.
// It calls the backend's metadata endpoint to retrieve a (possibly presigned) download URL.
func (s *ogaService) GetDownloadURL(ctx context.Context, key string) (string, error) {
	apiURL := fmt.Sprintf("uploads/%s", key)
	resp, err := s.httpClient.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch upload metadata: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		slog.WarnContext(ctx, "failed to fetch upload metadata, falling back to local content URL",
			"key", key, "status", resp.Status)
		return "", fmt.Errorf("failed to fetch upload metadata, status code: %d", resp.StatusCode)
	}

	var metadata struct {
		DownloadURL string `json:"download_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return "", fmt.Errorf("failed to decode upload metadata: %w", err)
	}

	if metadata.DownloadURL == "" {
		return "", fmt.Errorf("metadata response missing download_url")
	}

	slog.InfoContext(ctx, "resolved download URL from metadata", "key", key, "downloadURL", metadata.DownloadURL)
	return metadata.DownloadURL, nil
}

// feedbackHistoryFromRaw converts the raw JSONB slice from the store into typed feedback entries.
func feedbackHistoryFromRaw(raw []map[string]any) []feedback.Entry {
	entries := make([]feedback.Entry, 0, len(raw))
	for _, m := range raw {
		b, err := json.Marshal(m)
		if err != nil {
			slog.Error("failed to marshal feedback history entry from raw", "error", err)
			continue
		}
		var e feedback.Entry
		if err := json.Unmarshal(b, &e); err != nil {
			slog.Error("failed to unmarshal feedback history entry", "error", err)
			continue
		}
		entries = append(entries, e)
	}
	return entries
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

	// Note: We use the authenticated httpClient here too, in case the serviceURL requires it.
	// If it shouldn't, we might need a separate client or use the raw http.Client inside.
	resp, err := s.httpClient.Post(serviceURL, "application/json", jsonData)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

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
