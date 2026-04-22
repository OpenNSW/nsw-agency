package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockOGAService is a mock implementation of OGAService for testing
type mockOGAService struct {
	// embed the interface so we don't have to implement everything
	OGAService

	mockCreateUploadURL func(ctx context.Context, payload []byte) (map[string]any, error)
	mockGetDownloadURL  func(ctx context.Context, key string) (map[string]any, error)
}

func (m *mockOGAService) CreateUploadURL(ctx context.Context, payload []byte) (map[string]any, error) {
	if m.mockCreateUploadURL != nil {
		return m.mockCreateUploadURL(ctx, payload)
	}
	return nil, nil
}

func (m *mockOGAService) GetDownloadURL(ctx context.Context, key string) (map[string]any, error) {
	if m.mockGetDownloadURL != nil {
		return m.mockGetDownloadURL(ctx, key)
	}
	return nil, nil
}

func TestHandleCreateUpload(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockSvc := &mockOGAService{
			mockCreateUploadURL: func(ctx context.Context, payload []byte) (map[string]any, error) {
				return map[string]any{
					"key":        "123-abc",
					"upload_url": "http://test/upload",
				}, nil
			},
		}
		handler, err := NewOGAHandler(mockSvc, 32<<20)
		if err != nil {
			t.Fatalf("failed to create handler: %v", err)
		}

		body := []byte(`{"filename":"test.txt","mime_type":"text/plain","size":123}`)
		req := httptest.NewRequest(http.MethodPost, "/api/oga/uploads", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()

		handler.HandleCreateUpload(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response body: %v", err)
		}

		if resp["key"] != "123-abc" {
			t.Errorf("expected key '123-abc', got %v", resp["key"])
		}
	})

	t.Run("invalid method", func(t *testing.T) {
		handler, err := NewOGAHandler(&mockOGAService{}, 32<<20)
		if err != nil {
			t.Fatalf("failed to create handler: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/api/oga/uploads", nil)
		rec := httptest.NewRecorder()

		handler.HandleCreateUpload(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("service error", func(t *testing.T) {
		mockSvc := &mockOGAService{
			mockCreateUploadURL: func(ctx context.Context, payload []byte) (map[string]any, error) {
				return nil, errors.New("upstream error")
			},
		}
		handler, err := NewOGAHandler(mockSvc, 32<<20)
		if err != nil {
			t.Fatalf("failed to create handler: %v", err)
		}

		body := []byte(`{"filename":"test.txt","mime_type":"text/plain","size":123}`)
		req := httptest.NewRequest(http.MethodPost, "/api/oga/uploads", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()

		handler.HandleCreateUpload(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})
}

func TestHandleGetUploadURL(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockSvc := &mockOGAService{
			mockGetDownloadURL: func(ctx context.Context, key string) (map[string]any, error) {
				return map[string]any{
					"download_url": "http://test/download",
					"expires_at":   float64(1234567890),
				}, nil
			},
		}
		handler, err := NewOGAHandler(mockSvc, 32<<20)
		if err != nil {
			t.Fatalf("failed to create handler: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/oga/uploads/550e8400-e29b-41d4-a716-446655440000.pdf", nil)
		req.SetPathValue("key", "550e8400-e29b-41d4-a716-446655440000.pdf") // Set the mux path value
		rec := httptest.NewRecorder()

		handler.HandleGetUploadURL(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response body: %v", err)
		}

		if resp["download_url"] != "http://test/download" {
			t.Errorf("expected download_url 'http://test/download', got %v", resp["download_url"])
		}
		if resp["expires_at"] != float64(1234567890) { // JSON unmarshals ints to float64
			t.Errorf("expected expires_at 1234567890, got %v", resp["expires_at"])
		}
	})
}

func TestNewOGAHandler(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		handler, err := NewOGAHandler(&mockOGAService{}, 32<<20)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if handler == nil {
			t.Fatalf("expected handler to be non-nil")
		}
		if handler.MaxRequestBytes != 32<<20 {
			t.Errorf("expected MaxRequestBytes %d, got %d", 32<<20, handler.MaxRequestBytes)
		}
	})

	t.Run("invalid config - negative", func(t *testing.T) {
		_, err := NewOGAHandler(&mockOGAService{}, -1)
		if err == nil {
			t.Error("expected error for negative MaxRequestBytes, got nil")
		}
	})

	t.Run("invalid config - zero", func(t *testing.T) {
		_, err := NewOGAHandler(&mockOGAService{}, 0)
		if err == nil {
			t.Error("expected error for zero MaxRequestBytes, got nil")
		}
	})
}
