package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/OpenNSW/nsw-agency/backend/internal/auth"
	"github.com/OpenNSW/nsw-agency/backend/internal/feedback"
	"github.com/OpenNSW/nsw-agency/backend/internal/rbac"
	"github.com/OpenNSW/nsw-agency/backend/internal/taskconfig"
	"github.com/OpenNSW/nsw-agency/backend/internal/template"
	"github.com/OpenNSW/nsw-agency/backend/pkg/httpclient"
	"gorm.io/gorm"
)

// ErrApplicationNotFound is returned when an application is not found
var ErrApplicationNotFound = errors.New("application not found")

// Service handles Agency portal operations
type Service interface {
	// CreateApplication creates a new application from injected data
	CreateApplication(ctx context.Context, req *InjectRequest) error

	// GetApplications returns a paginated list of applications (optionally filtered by status, consignment, or search)
	GetApplications(ctx context.Context, status string, consignmentID string, search string, page, pageSize int) (*PagedResponse[Application], error)

	// GetConsignments returns a paginated list of unique consignments with their latest status (optionally filtered by search)
	GetConsignments(ctx context.Context, search string, page, pageSize int) (*PagedResponse[ConsignmentSummary], error)

	// GetApplication returns a specific application by task ID
	GetApplication(ctx context.Context, taskID string) (*Application, error)

	// ReviewApplication approves or rejects an application and sends response back to service
	ReviewApplication(ctx context.Context, taskID string, reviewerData map[string]any) error

	// FeedbackApplication sends a change-request feedback to the trader via the NSW task API
	// and updates the application status to FEEDBACK_REQUESTED.
	FeedbackApplication(ctx context.Context, taskID string, content map[string]any) error

	// KeyExists checks if the key exists and the current authenticated user has permission to access it.
	KeyExists(ctx context.Context, key string) (bool, error)

	// TrackUpload registers a newly uploaded file key associated with the current user.
	TrackUpload(ctx context.Context, key string) error

	// Close closes the service and releases resources
	Close() error
}

// InjectRequest represents the incoming data from services
type InjectRequest struct {
	TaskID                string           `json:"taskId"`
	TaskCode              string           `json:"taskCode"`
	ConsignmentID         string           `json:"consignmentId"`
	Data                  map[string]any   `json:"data"`
	ServiceURL            string           `json:"serviceUrl"` // URL to send response back to
	AgencyFeedbackHistory []map[string]any `json:"agencyFeedbackHistory,omitempty"`
}

// Application represents an application for display in the UI
type Application struct {
	TaskID           string         `json:"taskId"`
	TaskCode         string         `json:"taskCode"`
	ConsignmentID    string         `json:"consignmentId"`
	ServiceURL       string         `json:"serviceUrl"`
	Data             map[string]any `json:"data"`                       // Data from NSW service to be rendered in the UI
	AgencyActionData map[string]any `json:"agencyActionData,omitempty"` // Copy of the payload sent back to the NSW after review, for display in the UI
	AllowedActions   []string       `json:"allowedActions,omitempty"`

	// Task metadata from config
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Category    string `json:"category,omitempty"`

	DataForm        json.RawMessage  `json:"dataForm,omitempty"`   // Schema for rendering the data in Read Only mode in the UI
	AgencyForm      json.RawMessage  `json:"agencyForm,omitempty"` // Schema for rendering the Agency Action form in the UI
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

// TaskResponse represents the Style B callback envelope sent back to the NSW service
type TaskResponse struct {
	Command string `json:"command"`
	Payload any    `json:"payload"`
}

type service struct {
	store            *ApplicationStore
	templateProvider template.Provider
	httpClient       *httpclient.Client
	roleService      *rbac.RoleService
}

// NewService creates a new Agency service instance with database storage
func NewService(store *ApplicationStore, templateProvider template.Provider, httpClient *httpclient.Client, roleService *rbac.RoleService) Service {
	if store == nil || templateProvider == nil || httpClient == nil || roleService == nil {
		panic("NewService: all dependencies must be non-nil")
	}
	return &service{
		store:            store,
		templateProvider: templateProvider,
		httpClient:       httpClient,
		roleService:      roleService,
	}
}

// CreateApplication creates a new application from injected data.
func (s *service) CreateApplication(ctx context.Context, req *InjectRequest) error {
	if req.TaskID == "" || req.TaskCode == "" || req.ConsignmentID == "" || req.ServiceURL == "" {
		return fmt.Errorf("missing required fields in InjectRequest")
	}

	existing, err := s.store.GetByTaskID(req.TaskID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to query existing application: %w", err)
		}
		// Record doesn't exist — fall through to create.
	} else if existing.Status == "FEEDBACK_REQUESTED" {
		slog.InfoContext(ctx, "trader resubmitted after feedback, resetting to PENDING", "taskID", req.TaskID)
		return s.store.UpdateDataAndResetStatus(req.TaskID, req.Data)
	}

	appRecord := &ApplicationRecord{
		TaskID:        req.TaskID,
		TaskCode:      req.TaskCode,
		ConsignmentID: req.ConsignmentID,
		ServiceURL:    req.ServiceURL,
		Data:          req.Data,
		Status:        "PENDING",
	}

	return s.store.CreateOrUpdate(appRecord)
}

// GetApplications returns a paginated list of applications
func (s *service) GetApplications(ctx context.Context, status string, consignmentID string, search string, page, pageSize int) (*PagedResponse[Application], error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	records, total, err := s.store.List(ctx, status, consignmentID, search, offset, pageSize)
	if err != nil {
		return nil, err
	}

	authCtx := auth.GetAuthContext(ctx)
	var roles []rbac.RoleRecord
	if authCtx != nil && authCtx.User != nil {
		var err error
		roles, err = s.roleService.GetRolesForUser(authCtx.User.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get roles for user: %w", err)
		}
	}

	applications := make([]Application, 0, len(records))
	for _, record := range records {
		var permissions []taskconfig.Permission
		app := Application{
			TaskID:        record.TaskID,
			TaskCode:      record.TaskCode,
			ConsignmentID: record.ConsignmentID,
			ServiceURL:    record.ServiceURL,
			Data:          record.Data,
			Status:        record.Status,
			ReviewedAt:    record.ReviewedAt,
			CreatedAt:     record.CreatedAt,
			UpdatedAt:     record.UpdatedAt,
		}

		if config, err := s.templateProvider.GetTaskConfig(record.TaskCode); err == nil {
			app.Title = config.Meta.Title
			app.Category = config.Meta.Category
			app.Icon = config.Meta.Icon
			permissions = config.Permissions
		}

		accessible, _ := resolveAccess(roles, permissions)
		if !accessible {
			continue
		}

		applications = append(applications, app)
	}

	return &PagedResponse[Application]{
		Items:    applications,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetConsignments returns a paginated list of unique consignments
func (s *service) GetConsignments(ctx context.Context, search string, page, pageSize int) (*PagedResponse[ConsignmentSummary], error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	summaries, total, err := s.store.ListConsignments(ctx, search, offset, pageSize)
	if err != nil {
		return nil, err
	}

	return &PagedResponse[ConsignmentSummary]{
		Items:    summaries,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetApplication returns a specific application by task ID
func (s *service) GetApplication(ctx context.Context, taskID string) (*Application, error) {
	record, err := s.store.GetByTaskID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApplicationNotFound
		}
		return nil, fmt.Errorf("failed to get application: %w", err)
	}

	authCtx := auth.GetAuthContext(ctx)
	var roles []rbac.RoleRecord
	if authCtx != nil && authCtx.User != nil {
		var err error
		roles, err = s.roleService.GetRolesForUser(authCtx.User.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get roles for user: %w", err)
		}
	}

	app := &Application{
		TaskID:           record.TaskID,
		TaskCode:         record.TaskCode,
		ConsignmentID:    record.ConsignmentID,
		ServiceURL:       record.ServiceURL,
		Data:             record.Data,
		AgencyActionData: record.ReviewerResponse,
		Status:           record.Status,
		FeedbackHistory:  record.AgencyFeedbackHistory,
		ReviewedAt:       record.ReviewedAt,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}

	// Attach task configuration
	config, err := s.templateProvider.GetTaskConfig(record.TaskCode)
	if err != nil {
		slog.WarnContext(ctx, "task config not found for application", "taskID", taskID, "taskCode", record.TaskCode)
		_, app.AllowedActions = resolveAccess(roles, nil)
	} else {
		app.Title = config.Meta.Title
		app.Description = config.Meta.Description
		app.Icon = config.Meta.Icon
		app.Category = config.Meta.Category

		_, app.AllowedActions = resolveAccess(roles, config.Permissions)

		if config.Forms.View != "" {
			if form, ok := s.templateProvider.GetForm(config.Forms.View); ok {
				app.DataForm = form
			} else {
				slog.WarnContext(ctx, "view form not found", "taskCode", record.TaskCode, "formID", config.Forms.View)
			}
		}
		if config.Forms.Review != "" {
			if form, ok := s.templateProvider.GetForm(config.Forms.Review); ok {
				app.AgencyForm = form
			} else {
				slog.WarnContext(ctx, "review form not found", "taskCode", record.TaskCode, "formID", config.Forms.Review)
			}
		}
	}

	return app, nil
}

// buildCallbackURL constructs the callback URL target.
func buildCallbackURL(serviceURL, taskID string) string {
	if strings.Contains(serviceURL, "{id}") {
		return strings.ReplaceAll(serviceURL, "{id}", url.PathEscape(taskID))
	}
	u, err := url.Parse(serviceURL)
	if err != nil {
		return fmt.Sprintf("%s/%s", strings.TrimSuffix(serviceURL, "/"), url.PathEscape(taskID))
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + taskID
	u.RawPath = ""
	return u.String()
}

// ReviewApplication approves or rejects an application
func (s *service) ReviewApplication(ctx context.Context, taskID string, reviewerResponse map[string]any) error {
	app, err := s.GetApplication(ctx, taskID)
	if err != nil {
		return err
	}

	command := "approve"
	if config, err := s.templateProvider.GetTaskConfig(app.TaskCode); err == nil && config.Behavior != nil {
		outcomeField := config.Behavior.OutcomeField
		if outcomeField == "" {
			outcomeField = taskconfig.DefaultOutcomeField
		}
		if outcome, ok := reviewerResponse[outcomeField].(string); ok && outcome != "" {
			command = outcome
		}
	} else {
		if outcome, ok := reviewerResponse[taskconfig.DefaultOutcomeField].(string); ok && outcome != "" {
			command = outcome
		}
	}

	// Build Style B payload: envelope containing "command" and nested "payload"
	response := TaskResponse{
		Command: command,
		Payload: reviewerResponse,
	}

	callbackURL := buildCallbackURL(app.ServiceURL, app.TaskID)

	if err := s.sendToService(ctx, callbackURL, response); err != nil {
		return fmt.Errorf("failed to send response to service: %w", err)
	}

	status := "DONE"
	if config, err := s.templateProvider.GetTaskConfig(app.TaskCode); err == nil && config.Behavior != nil && config.Behavior.StatusMap != nil {
		outcomeField := config.Behavior.OutcomeField
		if outcomeField == "" {
			outcomeField = taskconfig.DefaultOutcomeField
		}
		if outcome, ok := reviewerResponse[outcomeField].(string); ok {
			if mappedStatus, ok := config.Behavior.StatusMap[outcome]; ok {
				status = mappedStatus
			}
		}
	}

	return s.store.UpdateStatus(taskID, status, reviewerResponse)
}

// FeedbackApplication sends Agency feedback to the trader
func (s *service) FeedbackApplication(ctx context.Context, taskID string, content map[string]any) error {
	app, err := s.GetApplication(ctx, taskID)
	if err != nil {
		return err
	}

	entry := feedback.Entry{
		Content:   content,
		Timestamp: time.Now().UTC(),
		Round:     len(app.FeedbackHistory) + 1,
	}

	// Build Style B payload for feedback/amendment
	response := TaskResponse{
		Command: "request-amendment",
		Payload: content,
	}

	callbackURL := buildCallbackURL(app.ServiceURL, app.TaskID)

	if err := s.sendToService(ctx, callbackURL, response); err != nil {
		return fmt.Errorf("failed to send feedback to service: %w", err)
	}

	return s.store.AppendFeedback(taskID, entry)
}

func (s *service) sendToService(ctx context.Context, callbackURL string, response TaskResponse) error {
	jsonData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// log the jsonData
	slog.Log(ctx, slog.LevelDebug, "sending response to service", "url", callbackURL, "payload", string(jsonData))

	resp, err := s.httpClient.Post(callbackURL, "application/json", jsonData)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.ErrorContext(ctx, "failed to close response body", "error", err)
		}
	}(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("service returned status %d", resp.StatusCode)
	}
	return nil
}

func resolveAccess(roles []rbac.RoleRecord, permissions []taskconfig.Permission) (bool, []string) {
	if len(permissions) == 0 {
		return true, []string{"VIEW", "REVIEW", "FEEDBACK"}
	}
	return rbac.ResolveAccess(roles, permissions)
}

func (s *service) Close() error {
	if s.store != nil {
		return s.store.Close()
	}
	return nil
}

func hasAction(actions []string, action string) bool {
	for _, a := range actions {
		if a == action {
			return true
		}
	}
	return false
}

func (s *service) KeyExists(ctx context.Context, key string) (bool, error) {
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return false, nil
	}

	// 1. Machine client check (M2M bypass)
	if authCtx.Client != nil {
		return true, nil
	}

	if authCtx.User == nil {
		return false, nil
	}

	// 2. Check uploaded_files first (ownership authorization)
	uploadedFile, err := s.store.GetUploadedFile(ctx, key)
	if err == nil {
		if uploadedFile.UploadedBy == authCtx.User.ID {
			return true, nil
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}

	// 3. Fallback to checking applications (rbac / access control check)
	records, err := s.store.FindApplicationsByFileKey(ctx, key)
	if err != nil {
		return false, err
	}
	if len(records) == 0 {
		return false, nil
	}

	roles, err := s.roleService.GetRolesForUser(authCtx.User.ID)
	if err != nil {
		return false, fmt.Errorf("failed to get user roles: %w", err)
	}

	for _, record := range records {
		var permissions []taskconfig.Permission
		if config, err := s.templateProvider.GetTaskConfig(record.TaskCode); err == nil {
			permissions = config.Permissions
		}
		accessible, allowedActions := resolveAccess(roles, permissions)
		if accessible && hasAction(allowedActions, "VIEW") {
			return true, nil
		}
	}

	return false, nil
}

func (s *service) TrackUpload(ctx context.Context, key string) error {
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil || authCtx.User == nil {
		return fmt.Errorf("unauthorized upload: user context not found")
	}
	return s.store.TrackUpload(ctx, key, authCtx.User.ID)
}
