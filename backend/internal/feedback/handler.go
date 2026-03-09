package feedback

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// Service is a narrow interface for feedback operations, avoiding a circular
// import with the parent internal package.
type Service interface {
	FeedbackApplication(ctx context.Context, taskID uuid.UUID, content map[string]any) error
}

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) HandleFeedback(w http.ResponseWriter, r *http.Request) {
	taskIDStr := r.PathValue("taskId")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid taskId format")
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

    feedback, ok := body["feedback"].(string)

	if !ok || strings.TrimSpace(feedback) == "" {
		writeJSONError(w, http.StatusBadRequest, "feedback field is required and must be a non-empty string")
		return
	}

	if err := h.service.FeedbackApplication(r.Context(), taskID, body); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to send feedback: "+err.Error())
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Feedback sent successfully",
	})
}

func writeJSONResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSONResponse(w, status, map[string]string{"error": message})
}
