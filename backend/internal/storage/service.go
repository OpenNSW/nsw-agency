// Package storage handles file storage operations including upload and download URL generation.
package storage

import (
	"context"
	"encoding/json"
	"errors"
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

var (
	ErrProhibitedFileType = errors.New("prohibited file type or extension")
	ErrDisallowedMimeType = errors.New("disallowed MIME type")
	ErrInvalidFilename    = errors.New("invalid or unsafe filename")
	ErrFileSizeExceeded   = errors.New("file size exceeds maximum allowed limit")
)

// Allowed MIME types whitelist for customs documents and attachments.
var allowedMimeTypes = map[string]struct{}{
	"application/pdf":    {},
	"image/jpeg":         {},
	"image/jpg":          {},
	"image/png":          {},
	"image/tiff":         {},
	"text/csv":           {},
	"text/plain":         {},
	"application/json":   {},
	"application/msword": {},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       {},
}

// Allowed extensions whitelist for customs documents and attachments.
var allowedExtensions = map[string]struct{}{
	".pdf":  {},
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".tiff": {},
	".tif":  {},
	".csv":  {},
	".txt":  {},
	".json": {},
	".doc":  {},
	".docx": {},
	".xls":  {},
	".xlsx": {},
}

// CleanFilename sanitizes input filename and validates its extension against allowed types.
func CleanFilename(filename string) (string, error) {
	if strings.Contains(filename, "\x00") {
		return "", fmt.Errorf("%w: filename contains null byte", ErrInvalidFilename)
	}

	cleanName := filepath.Base(filepath.Clean(filename))
	if cleanName == "." || cleanName == "/" || cleanName == "\\" || cleanName == "" {
		return "", fmt.Errorf("%w: unsafe or empty filename", ErrInvalidFilename)
	}

	ext := strings.ToLower(filepath.Ext(cleanName))
	if ext == "" {
		return "", fmt.Errorf("%w: file extension required", ErrInvalidFilename)
	}

	if _, allowed := allowedExtensions[ext]; !allowed {
		return "", fmt.Errorf("%w: extension %s is not permitted", ErrProhibitedFileType, ext)
	}

	return cleanName, nil
}

// validateUploadRequest performs multi-layered security validation on upload requests.
func validateUploadRequest(req *UploadRequest) error {
	if req.Filename == "" || req.MimeType == "" || req.Size <= 0 {
		return fmt.Errorf("invalid upload request: missing required fields")
	}

	cleanName, err := CleanFilename(req.Filename)
	if err != nil {
		return err
	}
	req.Filename = cleanName

	// Maximum single file size limit: 50MB
	const maxFileSize int64 = 50 << 20
	if req.Size > maxFileSize {
		return fmt.Errorf("%w: %d bytes (max %d bytes)", ErrFileSizeExceeded, req.Size, maxFileSize)
	}

	mimeClean := strings.ToLower(strings.TrimSpace(strings.Split(req.MimeType, ";")[0]))
	if _, allowed := allowedMimeTypes[mimeClean]; !allowed {
		return fmt.Errorf("%w: %s", ErrDisallowedMimeType, req.MimeType)
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
