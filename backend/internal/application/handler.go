package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/OpenNSW/nsw-agency/backend/pkg/httputil"
)

// OGAHandler handles HTTP requests for OGA portal operations
type OGAHandler struct {
	service         OGAService
	MaxRequestBytes int64
}

// NewOGAHandler creates a new OGA handler instance
func NewOGAHandler(service OGAService, maxRequestBytes int64) (*OGAHandler, error) {
	if maxRequestBytes <= 0 {
		return nil, fmt.Errorf("invalid MaxRequestBytes: %d (must be greater than 0)", maxRequestBytes)
	}
	return &OGAHandler{
		service:         service,
		MaxRequestBytes: maxRequestBytes,
	}, nil
}

// parseTaskID extracts the taskId from the request path.
func (h *OGAHandler) parseTaskID(w http.ResponseWriter, r *http.Request) (string, error) {
	taskIDStr := r.PathValue("taskId")
	if taskIDStr == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "taskId is required")
		return "", errors.New("taskId is required")
	}
	return taskIDStr, nil
}

// HandleInjectData handles POST /api/oga/inject
// This is the endpoint that external services use to inject data into OGA portal
func (h *OGAHandler) HandleInjectData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()

	var req InjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Create application in database
	if err := h.service.CreateApplication(ctx, &req); err != nil {
		slog.ErrorContext(ctx, "failed to create application", "error", err)
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to create application: "+err.Error())
		return
	}

	slog.InfoContext(ctx, "data injected successfully",
		"taskID", req.TaskID,
		"workflowID", req.WorkflowID)

	httputil.WriteJSONResponse(w, http.StatusCreated, map[string]any{
		"success": true,
		"message": "Data injected successfully",
		"taskId":  req.TaskID,
	})
}

// HandleGetApplications handles GET /api/oga/applications
// Returns all applications, optionally filtered by status, workflowId, or q query parameter
func (h *OGAHandler) HandleGetApplications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()
	status := r.URL.Query().Get("status")
	workflowID := r.URL.Query().Get("workflowId")
	search := r.URL.Query().Get("q")

	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil && r.URL.Query().Get("page") != "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid page number")
		return
	}
	pageSize, err := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if err != nil && r.URL.Query().Get("pageSize") != "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid page size")
		return
	}

	result, err := h.service.GetApplications(ctx, status, workflowID, search, page, pageSize)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get applications", "error", err)
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to get applications")
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, result)
}

// HandleGetWorkflows handles GET /api/oga/workflows
// Returns a paginated list of unique workflows with their latest status, optionally filtered by q
func (h *OGAHandler) HandleGetWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()
	search := r.URL.Query().Get("q")

	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil && r.URL.Query().Get("page") != "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid page number")
		return
	}
	pageSize, err := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if err != nil && r.URL.Query().Get("pageSize") != "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid page size")
		return
	}

	result, err := h.service.GetWorkflows(ctx, search, page, pageSize)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get workflows", "error", err)
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to get workflows")
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, result)
}

// HandleGetApplication handles GET /api/oga/applications/{taskId}
// Returns a specific application by task ID
func (h *OGAHandler) HandleGetApplication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	taskID, err := h.parseTaskID(w, r)
	if err != nil {
		return
	}

	ctx := r.Context()
	application, err := h.service.GetApplication(ctx, taskID)
	if err != nil {
		if errors.Is(err, ErrApplicationNotFound) {
			httputil.WriteJSONError(w, http.StatusNotFound, "Application not found")
		} else {
			slog.ErrorContext(ctx, "failed to get application",
				"taskID", taskID,
				"error", err)
			httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to get application")
		}
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, application)
}

// HandleHealth handles GET /health
// Simple health check endpoint
func (h *OGAHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSONResponse(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "oga-portal",
	})
}

// HandleReviewApplication handles POST /api/oga/applications/{taskId}/review
// Called when OGA officer approves/rejects an application
// Sends the response back to the originating service
func (h *OGAHandler) HandleReviewApplication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	taskID, err := h.parseTaskID(w, r)
	if err != nil {
		return
	}

	ctx := r.Context()

	// Parse request body
	var requestBody map[string]any

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Process review and send response to service
	if err := h.service.ReviewApplication(ctx, taskID, requestBody); err != nil {
		if errors.Is(err, ErrApplicationNotFound) {
			httputil.WriteJSONError(w, http.StatusNotFound, "Application not found")
		} else {
			slog.ErrorContext(ctx, "failed to review application",
				"taskID", taskID,
				"error", err)
			httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to review application: "+err.Error())
		}
		return
	}

	slog.InfoContext(ctx, "application reviewed",
		"taskID", taskID,
	)

	httputil.WriteJSONResponse(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Application reviewed successfully",
	})
}
