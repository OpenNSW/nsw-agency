package storage

import (
	"context"
	"errors"
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

func TestCleanFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"document.pdf", "document.pdf", false},
		{"../../etc/passwd.pdf", "passwd.pdf", false},
		{"malware.exe", "", true},
		{"script.sh", "", true},
		{"shell.php", "", true},
		{"page.html", "", true},
		{"vector.svg", "", true},
		{"macro.xls", "macro.xls", false},
		{"file\x00name.pdf", "", true},
	}

	for _, tt := range tests {
		res, err := CleanFilename(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("CleanFilename(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if !tt.wantErr && res != tt.expected {
			t.Errorf("CleanFilename(%q) = %q, expected %q", tt.input, res, tt.expected)
		}
	}
}

func TestValidateUploadRequest(t *testing.T) {
	tests := []struct {
		name      string
		req       UploadRequest
		targetErr error
		wantErr   bool
	}{
		{
			name: "valid PDF upload",
			req: UploadRequest{
				Filename: "document.pdf",
				MimeType: "application/pdf",
				Size:     1024,
			},
			wantErr: false,
		},
		{
			name: "valid PNG image upload",
			req: UploadRequest{
				Filename: "image.png",
				MimeType: "image/png",
				Size:     2048,
			},
			wantErr: false,
		},
		{
			name: "disallowed executable extension .exe",
			req: UploadRequest{
				Filename: "malware.exe",
				MimeType: "application/octet-stream",
				Size:     1024,
			},
			targetErr: ErrProhibitedFileType,
			wantErr:   true,
		},
		{
			name: "disallowed script extension .php",
			req: UploadRequest{
				Filename: "shell.php",
				MimeType: "text/plain",
				Size:     512,
			},
			targetErr: ErrProhibitedFileType,
			wantErr:   true,
		},
		{
			name: "disallowed script extension .sh",
			req: UploadRequest{
				Filename: "script.sh",
				MimeType: "text/plain",
				Size:     256,
			},
			targetErr: ErrProhibitedFileType,
			wantErr:   true,
		},
		{
			name: "disallowed HTML extension",
			req: UploadRequest{
				Filename: "phish.html",
				MimeType: "text/plain",
				Size:     1024,
			},
			targetErr: ErrProhibitedFileType,
			wantErr:   true,
		},
		{
			name: "disallowed MIME type",
			req: UploadRequest{
				Filename: "document.pdf",
				MimeType: "audio/mpeg",
				Size:     1024,
			},
			targetErr: ErrDisallowedMimeType,
			wantErr:   true,
		},
		{
			name: "exceeds maximum size limit",
			req: UploadRequest{
				Filename: "huge.pdf",
				MimeType: "application/pdf",
				Size:     100 << 20, // 100MB > 50MB
			},
			targetErr: ErrFileSizeExceeded,
			wantErr:   true,
		},
		{
			name: "missing filename",
			req: UploadRequest{
				Filename: "",
				MimeType: "application/pdf",
				Size:     1024,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUploadRequest(&tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateUploadRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.targetErr != nil && !errors.Is(err, tt.targetErr) {
				t.Errorf("validateUploadRequest() error = %v, expected target error %v", err, tt.targetErr)
			}
		})
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
