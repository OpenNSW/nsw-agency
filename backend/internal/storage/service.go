// Package storage handles file storage operations including upload and download URL generation.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/OpenNSW/nsw-agency/backend/pkg/httpclient"
)

// UploadRequest represents the payload sent by the frontend to initiate a file upload.
type UploadRequest struct {
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

// FileMetadata mirrors the backend's uploads.FileMetadata struct.
// It represents the full metadata of an uploaded file as returned by the main backend.
type FileMetadata struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Key       string `json:"key"`
	URL       string `json:"url,omitempty"`
	UploadURL string `json:"upload_url,omitempty"`
	Size      int64  `json:"size"`
	MimeType  string `json:"mime_type"`
}

// DownloadMetadata represents the response returned when a download URL is fetched.
type DownloadMetadata struct {
	DownloadURL string `json:"download_url"`
	ExpiresAt   int64  `json:"expires_at"`
}

const storageBasePath = "storage"

// Service handles storage operations (upload/download URLs)
type Service interface {
	// GetDownloadURL fetches a download URL for a key from the main backend.
	GetDownloadURL(ctx context.Context, key string) (*DownloadMetadata, error)

	// CreateUploadURL proxies an upload initialization request to the main backend.
	CreateUploadURL(ctx context.Context, req UploadRequest) (*FileMetadata, error)
}

type service struct {
	httpClient *httpclient.Client
}

// NewService creates a new storage service instance
func NewService(httpClient *httpclient.Client) Service {
	return &service{
		httpClient: httpClient,
	}
}

// GetDownloadURL returns a download URL for a file stored in the main backend.
// It calls the backend's metadata endpoint to retrieve a (possibly presigned) download URL.
func (s *service) GetDownloadURL(ctx context.Context, key string) (*DownloadMetadata, error) {
	apiURL := fmt.Sprintf("%s/%s", storageBasePath, url.PathEscape(key))
	resp, err := s.httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch upload metadata: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		slog.WarnContext(ctx, "failed to fetch upload metadata",
			"key", key, "status", resp.Status)
		return nil, fmt.Errorf("failed to fetch upload metadata, status code: %d", resp.StatusCode)
	}

	var metadata DownloadMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode upload metadata: %w", err)
	}

	if metadata.DownloadURL == "" {
		return nil, fmt.Errorf("metadata response missing download_url")
	}

	slog.InfoContext(ctx, "resolved download URL from metadata", "key", key, "downloadURL", metadata.DownloadURL)
	return &metadata, nil
}

// Allowed MIME types whitelist for customs documents and attachments.
var allowedMimeTypes = map[string]bool{
	"application/pdf":    true,
	"image/jpeg":         true,
	"image/jpg":          true,
	"image/png":          true,
	"image/tiff":         true,
	"text/csv":           true,
	"text/plain":         true,
	"application/json":   true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true,
	"application/octet-stream": true,
}

// Blocked dangerous and executable extensions.
var blockedExtensions = map[string]bool{
	".exe":  true,
	".sh":   true,
	".php":  true,
	".js":   true,
	".html": true,
	".htm":  true,
	".svg":  true,
	".bat":  true,
	".cmd":  true,
	".py":   true,
	".rb":   true,
	".jar":  true,
	".jsp":  true,
	".asp":  true,
	".aspx": true,
	".vbs":  true,
	".ps1":  true,
}

// validateUploadRequest performs multi-layered security validation on upload requests.
func validateUploadRequest(req *UploadRequest) error {
	if req.Filename == "" || req.MimeType == "" || req.Size <= 0 {
		return fmt.Errorf("invalid upload request: missing required fields")
	}

	if strings.Contains(req.Filename, "\x00") {
		return fmt.Errorf("invalid upload request: filename contains null byte")
	}

	// Sanitize filename to prevent path traversal
	req.Filename = filepath.Base(filepath.Clean(req.Filename))
	if req.Filename == "." || req.Filename == "/" || req.Filename == "\\" || req.Filename == "" {
		return fmt.Errorf("invalid upload request: unsafe filename")
	}

	// Maximum single file size limit: 50MB
	const maxFileSize int64 = 50 << 20
	if req.Size > maxFileSize {
		return fmt.Errorf("file size exceeds maximum allowed limit of %d bytes", maxFileSize)
	}

	ext := strings.ToLower(filepath.Ext(req.Filename))
	if ext == "" {
		return fmt.Errorf("file extension required")
	}

	if blockedExtensions[ext] {
		return fmt.Errorf("disallowed file extension: %s", ext)
	}

	mimeClean := strings.ToLower(strings.TrimSpace(strings.Split(req.MimeType, ";")[0]))
	if !allowedMimeTypes[mimeClean] {
		return fmt.Errorf("disallowed MIME type: %s", req.MimeType)
	}

	return nil
}

// CreateUploadURL proxies an upload initialization request to the main backend.
func (s *service) CreateUploadURL(ctx context.Context, req UploadRequest) (*FileMetadata, error) {
	if err := validateUploadRequest(&req); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal upload request: %w", err)
	}

	resp, err := s.httpClient.Post(storageBasePath, "application/json", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to POST upload metadata: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		decodeErr := json.NewDecoder(resp.Body).Decode(&errResp)
		errMsg := errResp["error"]
		if decodeErr != nil || errMsg == "" {
			errMsg = "unknown upstream error or invalid JSON response"
		}
		slog.WarnContext(ctx, "failed to fetch upload metadata from backend", "status", resp.Status, "error", errMsg)
		return nil, fmt.Errorf("backend error (status %d): %s", resp.StatusCode, errMsg)
	}

	var metadata FileMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode upload metadata: %w", err)
	}

	return &metadata, nil
}
