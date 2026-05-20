package application

import (
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
