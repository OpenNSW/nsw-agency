package user

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/OpenNSW/nsw-agency/backend/pkg/httputil"
)

// ProfileHandler handles HTTP requests for user profile operations.
type ProfileHandler struct {
	service *ProfileService
}

// NewProfileHandler creates a ProfileHandler.
func NewProfileHandler(service *ProfileService) *ProfileHandler {
	return &ProfileHandler{service: service}
}

// HandleMe handles GET /api/v1/users/me
func (h *ProfileHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()
	result, err := h.service.GetMe(ctx)
	if err != nil {
		if errors.Is(err, ErrUnauthenticated) {
			httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthenticated")
		} else {
			slog.ErrorContext(ctx, "failed to get user profile", "error", err)
			httputil.WriteJSONError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, result)
}
