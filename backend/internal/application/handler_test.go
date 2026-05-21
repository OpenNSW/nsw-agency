package application

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockService is a mock implementation of Service for testing
type mockService struct {
	// embed the interface so we don't have to implement everything
	Service
}

func TestNewHandler(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		handler, err := NewHandler(&mockService{}, 32<<20)
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
		_, err := NewHandler(&mockService{}, -1)
		if err == nil {
			t.Error("expected error for negative MaxRequestBytes, got nil")
		}
	})

	t.Run("invalid config - zero", func(t *testing.T) {
		_, err := NewHandler(&mockService{}, 0)
		if err == nil {
			t.Error("expected error for zero MaxRequestBytes, got nil")
		}
	})
}

func TestHandleInjectData_BodyTooLarge(t *testing.T) {
	handler, err := NewHandler(&mockService{}, 10)
	if err != nil {
		t.Fatalf("unexpected error creating handler: %v", err)
	}

	// Valid JSON prefix that forces the decoder to read past the 10-byte limit.
	body := strings.NewReader(`{"key":"` + strings.Repeat("a", 100) + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/nsw-agency/inject", body)
	w := httptest.NewRecorder()

	handler.HandleInjectData(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status %d, got %d", http.StatusRequestEntityTooLarge, w.Code)
	}
}

func TestHandleReviewApplication_BodyTooLarge(t *testing.T) {
	handler, err := NewHandler(&mockService{}, 10)
	if err != nil {
		t.Fatalf("unexpected error creating handler: %v", err)
	}

	// Valid JSON prefix that forces the decoder to read past the 10-byte limit.
	body := strings.NewReader(`{"key":"` + strings.Repeat("a", 100) + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/nsw-agency/applications/task-123/review", body)
	req.SetPathValue("taskId", "task-123")
	w := httptest.NewRecorder()

	handler.HandleReviewApplication(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status %d, got %d", http.StatusRequestEntityTooLarge, w.Code)
	}
}
