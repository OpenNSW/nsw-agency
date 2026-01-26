package oga

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/OpenNSW/nsw/internal/task"
	"github.com/OpenNSW/nsw/internal/workflow/model"
	"github.com/OpenNSW/nsw/utils"
	"github.com/google/uuid"
)

// OGAHandler handles HTTP requests for OGA portal operations
type OGAHandler struct {
	service     OGAService
	taskManager task.TaskManager
}

// NewOGAHandler creates a new OGA handler instance
func NewOGAHandler(service OGAService, taskManager task.TaskManager) *OGAHandler {
	return &OGAHandler{
		service:     service,
		taskManager: taskManager,
	}
}

// parseTaskID extracts and parses the taskId from the request path
func (h *OGAHandler) parseTaskID(w http.ResponseWriter, r *http.Request) (uuid.UUID, error) {
	taskIDStr := r.PathValue("taskId")
	if taskIDStr == "" {
		utils.WriteJSONError(w, http.StatusBadRequest, "taskId is required")
		return uuid.Nil, errors.New("taskId is required")
	}

	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		utils.WriteJSONError(w, http.StatusBadRequest, "invalid taskId format")
		return uuid.Nil, err
	}
	return taskID, nil
}

// HandleGetApplications handles GET /api/oga/applications
// Returns all applications ready for OGA review
func (h *OGAHandler) HandleGetApplications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()
	applications, err := h.service.GetApplications(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get applications", "error", err)
		utils.WriteJSONError(w, http.StatusInternalServerError, "Failed to get applications")
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, applications)
}

// HandleGetApplication handles GET /api/oga/applications/{taskId}
// Returns a specific application by task ID
func (h *OGAHandler) HandleGetApplication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
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
			utils.WriteJSONError(w, http.StatusNotFound, "Application not found")
		} else {
			slog.ErrorContext(ctx, "failed to get application",
				"taskID", taskID,
				"error", err)
			utils.WriteJSONError(w, http.StatusInternalServerError, "Failed to get application")
		}
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, application)
}

// HandleApproveApplication handles POST /api/oga/applications/{taskId}/approve
// OGA officer approves an application and submits form data
func (h *OGAHandler) HandleApproveApplication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	taskID, err := h.parseTaskID(w, r)
	if err != nil {
		return
	}

	var req struct {
		FormData     map[string]interface{} `json:"formData"`     // Complete form data (trader + OGA merged)
		Decision     string                 `json:"decision"`     // "APPROVED" or "REJECTED"
		ReviewerName string                 `json:"reviewerName"`
		Comments     string                 `json:"comments"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if req.Decision != "APPROVED" && req.Decision != "REJECTED" {
		utils.WriteJSONError(w, http.StatusBadRequest, "decision must be APPROVED or REJECTED")
		return
	}

	ctx := r.Context()

	// Enrich form data with reviewer metadata
	if req.FormData == nil {
		req.FormData = make(map[string]interface{})
	}
	req.FormData["reviewerName"] = req.ReviewerName
	req.FormData["comments"] = req.Comments
	req.FormData["decision"] = req.Decision

	// Call Task Manager callback with full form data
	if h.taskManager != nil {
		if err := h.taskManager.OnOGAFormSubmitted(ctx, taskID, req.FormData); err != nil {
			slog.ErrorContext(ctx, "failed to notify TaskManager of OGA form submission",
				"taskID", taskID,
				"error", err)
			utils.WriteJSONError(w, http.StatusInternalServerError, "Failed to submit form to Task Manager")
			return
		}
	}

	// Remove application from ready list
	if err := h.service.RemoveApplication(ctx, taskID); err != nil {
		slog.WarnContext(ctx, "failed to remove application from list",
			"taskID", taskID,
			"error", err)
		// Don't fail the request, form is already submitted
	}

	// Return success response
	utils.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Application " + req.Decision + " successfully",
	})
}

// HandleNotification handles POST /api/oga/notifications
// Receives notifications from Task Manager when applications are ready for review
func (h *OGAHandler) HandleNotification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var notification model.OGATaskNotification

	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		utils.WriteJSONError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	ctx := r.Context()

	// Add application to service
	if err := h.service.AddApplication(ctx, notification); err != nil {
		slog.ErrorContext(ctx, "failed to add application",
			"taskID", notification.TaskID,
			"error", err)
		utils.WriteJSONError(w, http.StatusInternalServerError, "Failed to add application")
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Application added for review",
	})
}
