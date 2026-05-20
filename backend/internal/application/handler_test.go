package application

import (
	"testing"
)

// mockOGAService is a mock implementation of OGAService for testing
type mockOGAService struct {
	// embed the interface so we don't have to implement everything
	OGAService
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
