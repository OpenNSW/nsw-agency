package storage

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/OpenNSW/nsw-agency/backend/pkg/httputil"
)

// KeyValidator checks whether a file key is authorized for access/download.
type KeyValidator interface {
	KeyExists(ctx context.Context, key string) (bool, error)
	TrackUpload(ctx context.Context, key string) error
}

// Handler handles HTTP requests for storage operations
type Handler struct {
	service         Service
	MaxRequestBytes int64
	keyValidator    KeyValidator
}

// NewHandler creates a new storage handler instance
func NewHandler(service Service, maxRequestBytes int64) *Handler {
	return &Handler{
		service:         service,
		MaxRequestBytes: maxRequestBytes,
	}
}

// WithKeyValidator attaches a KeyValidator to the Handler.
func (h *Handler) WithKeyValidator(kv KeyValidator) *Handler {
	h.keyValidator = kv
	return h
}

// HandleGetUploadURL returns a download URL for a file stored in the main backend.
func (h *Handler) HandleGetUploadURL(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "key is required")
		return
	}

	if h.keyValidator != nil {
		allowed, err := h.keyValidator.KeyExists(r.Context(), key)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to validate key ownership", "key", key, "error", err)
			httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to validate key ownership")
			return
		}
		if !allowed {
			slog.WarnContext(r.Context(), "unauthorized access attempt to storage key", "key", key)
			httputil.WriteJSONError(w, http.StatusForbidden, "Forbidden")
			return
		}
	}

	metadata, err := h.service.GetDownloadURL(r.Context(), key)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get download URL from backend", "key", key, "error", err)
		httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to get download URL")
		return
	}

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
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

	result, err := h.service.CreateUploadURL(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create upload URL", "error", err)
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to create upload URL")
		return
	}

	if h.keyValidator != nil && result != nil && result.Key != "" {
		if err := h.keyValidator.TrackUpload(r.Context(), result.Key); err != nil {
			slog.ErrorContext(r.Context(), "failed to track upload key", "key", result.Key, "error", err)
		}
	}

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	httputil.WriteJSONResponse(w, http.StatusOK, result)
}
