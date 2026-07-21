package storage

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/OpenNSW/nsw-agency/backend/pkg/httputil"
)

// KeyValidator defines the interface for checking if a storage key exists in applications.
type KeyValidator interface {
	KeyExists(ctx context.Context, key string) (bool, error)
	TrackUpload(ctx context.Context, key string) error
}

// Handler handles HTTP requests for storage operations
type Handler struct {
	service         Service
	MaxRequestBytes int64
	KeyValidator    KeyValidator
}

// NewHandler creates a new storage handler instance
func NewHandler(service Service, maxRequestBytes int64) *Handler {
	return &Handler{
		service:         service,
		MaxRequestBytes: maxRequestBytes,
	}
}

// WithKeyValidator registers a KeyValidator on the handler
func (h *Handler) WithKeyValidator(kv KeyValidator) *Handler {
	h.KeyValidator = kv
	return h
}

// HandleGetUploadURL returns a download URL for a file stored in the main backend.
func (h *Handler) HandleGetUploadURL(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "key is required")
		return
	}

	if h.KeyValidator == nil {
		slog.ErrorContext(r.Context(), "storage handler: KeyValidator is not configured")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	exists, err := h.KeyValidator.KeyExists(r.Context(), key)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to validate storage key", "key", key, "error", err)
		httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to validate key")
		return
	}
	if !exists {
		slog.WarnContext(r.Context(), "unauthorized download attempt: key not found in agency applications", "key", key)
		httputil.WriteJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	metadata, err := h.service.GetDownloadURL(r.Context(), key)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get download URL from backend", "key", key, "error", err)
		httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to get download URL")
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, metadata)
}

// HandleCreateUpload prepares an upload by requesting an upload URL from the main backend.
func (h *Handler) HandleCreateUpload(w http.ResponseWriter, r *http.Request) {
	// TODO: Add Authentication & Authorization middleware
	// Access must be restricted to authorized Agency officers to prevent unauthorized users
	// from generating proxy pre-signed upload URLs. Introduce a configuration flag (e.g. AGENCY_DISABLE_AUTH)
	// to make bypassing explicit for specific environments

	r.Body = http.MaxBytesReader(w, r.Body, h.MaxRequestBytes)
	var req UploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if h.KeyValidator == nil {
		slog.ErrorContext(r.Context(), "storage handler: KeyValidator is not configured")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	result, err := h.service.CreateUploadURL(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create upload URL", "error", err)
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to create upload URL")
		return
	}

	if err := h.KeyValidator.TrackUpload(r.Context(), result.Key); err != nil {
		slog.ErrorContext(r.Context(), "failed to track upload key", "key", result.Key, "error", err)
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to create upload URL")
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, result)
}
