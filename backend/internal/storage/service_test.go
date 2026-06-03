package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenNSW/nsw-agency/backend/pkg/httpclient"
)

func TestService_CreateUploadURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/storage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"key":"123-abc", "name":"test.txt", "upload_url":"http://test/upload"}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := httpclient.NewClientBuilder().
		WithBaseURL(server.URL + "/").
		Build()

	service := NewService(client)

	req := UploadRequest{
		Filename: "test.txt",
		MimeType: "text/plain",
		Size:     123,
	}
	ctx := context.Background()

	result, err := service.CreateUploadURL(ctx, req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Key != "123-abc" {
		t.Errorf("expected key '123-abc', got %v", result.Key)
	}
	if result.UploadURL != "http://test/upload" {
		t.Errorf("expected upload_url 'http://test/upload', got %v", result.UploadURL)
	}
}

func TestService_GetDownloadURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/storage/550e8400-e29b-41d4-a716-446655440000.pdf", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"download_url":"http://test/download", "expires_at": 1234567890}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := httpclient.NewClientBuilder().
		WithBaseURL(server.URL + "/").
		Build()

	service := NewService(client)
	ctx := context.Background()

	metadata, err := service.GetDownloadURL(ctx, "550e8400-e29b-41d4-a716-446655440000.pdf")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if metadata.DownloadURL != "http://test/download" {
		t.Errorf("expected download_url 'http://test/download', got %v", metadata.DownloadURL)
	}
	if metadata.ExpiresAt != 1234567890 {
		t.Errorf("expected expires_at 1234567890, got %v", metadata.ExpiresAt)
	}
}
