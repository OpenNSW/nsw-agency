package httpclient

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNoAuthenticator(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient("", 5*time.Second, nil)

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.Status)
	}
}

func TestPost(t *testing.T) {
	expectedBody := "hello world"
	expectedContentType := "text/plain"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %v", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != expectedContentType {
			t.Errorf("expected content type %q, got %q", expectedContentType, ct)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != expectedBody {
			t.Errorf("expected body %q, got %q", expectedBody, string(body))
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient("", 5*time.Second, nil)
	resp, err := client.Post(server.URL, expectedContentType, []byte(expectedBody))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status Created, got %v", resp.Status)
	}
}

// MockAuthenticator is a helper for testing authentication failures.
type MockAuthenticator struct {
	err error
}

func (m *MockAuthenticator) Authenticate(req *http.Request) error {
	return m.err
}

func TestDoAuthenticationFailure(t *testing.T) {
	authErr := fmt.Errorf("auth failed")
	auth := &MockAuthenticator{err: authErr}
	client := NewClient("", 5*time.Second, auth)

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	resp, err := client.Do(req)

	if err != authErr {
		t.Errorf("expected error %v, got %v", authErr, err)
	}
	if resp != nil {
		t.Error("expected nil response on auth failure")
	}
}

func TestGetInvalidURL(t *testing.T) {
	client := NewClient("", 5*time.Second, nil)
	_, err := client.Get(":") // Invalid URL
	if err == nil {
		t.Error("expected error for invalid URL in Get")
	}
}

func TestGetResolveURLError(t *testing.T) {
	client := &Client{BaseURL: "http://a b.com"}
	_, err := client.Get("path")
	if err == nil {
		t.Error("expected error for invalid baseURL in Get")
	}
}

func TestPostResolveURLError(t *testing.T) {
	client := &Client{BaseURL: "http://a b.com"}
	_, err := client.Post("path", "text/plain", nil)
	if err == nil {
		t.Error("expected error for invalid baseURL in Post")
	}
}

func TestPostInvalidURL(t *testing.T) {
	client := NewClient("", 5*time.Second, nil)
	_, err := client.Post("http://[::1]:80%2g/", "text/plain", nil) // Invalid URL
	if err == nil {
		t.Error("expected error for invalid URL in Post")
	}
}

func TestResolveURLPathError(t *testing.T) {
	client := NewClient("http://example.com", 1*time.Second, nil)
	// Triggering url.Parse error on path is hard, but let's try something with control characters
	_, _ = client.resolveURL("http://[::1]:80%2g/")
	// Wait, if path has http:// prefix it returns early.
	// Let's try a path that is not absolute but has invalid characters
	_, err := client.resolveURL("path\x7f")
	if err == nil {
		t.Error("expected error for invalid path in resolveURL")
	}
}

func TestNewClientBaseURL(t *testing.T) {
	tests := []struct {
		baseURL  string
		expected string
	}{
		{"http://example.com", "http://example.com/"},
		{"http://example.com/", "http://example.com/"},
		{"", ""},
	}

	for _, tc := range tests {
		client := NewClient(tc.baseURL, 1*time.Second, nil)
		if client.BaseURL != tc.expected {
			t.Errorf("expected BaseURL %q, got %q", tc.expected, client.BaseURL)
		}
	}
}

func TestResolveURL(t *testing.T) {
	tests := []struct {
		baseURL  string
		path     string
		expected string
	}{
		{"http://api.com/", "v1/resource", "http://api.com/v1/resource"},
		{"http://api.com/", "/v1/resource", "http://api.com/v1/resource"},
		{"http://api.com", "v1/resource", "http://api.com/v1/resource"},
		{"http://api.com", "/v1/resource", "http://api.com/v1/resource"},
		{"", "http://other.com/api", "http://other.com/api"},
		{"http://api.com/", "http://other.com/api", "http://other.com/api"},
		{"http://api.com/", "https://other.com/api", "https://other.com/api"},
	}

	for _, tc := range tests {
		client := NewClient(tc.baseURL, 1*time.Second, nil)
		got, err := client.resolveURL(tc.path)
		if err != nil {
			t.Errorf("unexpected error for baseURL %q and path %q: %v", tc.baseURL, tc.path, err)
			continue
		}
		if got != tc.expected {
			t.Errorf("for baseURL %q and path %q: expected %q, got %q", tc.baseURL, tc.path, tc.expected, got)
		}
	}
}

func TestResolveURLError(t *testing.T) {
	tests := []struct {
		baseURL string
		path    string
	}{
		{"http://a b.com", "path"}, // Invalid baseURL
		// For path, url.Parse rarely fails unless it's a very specific invalid string
		// Let's try to trigger the baseURL parse error
	}

	for _, tc := range tests {
		client := &Client{BaseURL: tc.baseURL}
		_, err := client.resolveURL(tc.path)
		if err == nil {
			t.Errorf("expected error for baseURL %q and path %q", tc.baseURL, tc.path)
		}
	}
}

func TestAuthLeakPrevention(t *testing.T) {
	apiKey := "secret-key"
	auth := NewAPIKeyAuthenticator(apiKey, "")
	baseURL := "https://api.trusted.com"
	client := NewClient(baseURL, 5*time.Second, auth)

	// Mock an external server
	externalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "" {
			t.Errorf("Security risk: Auth header leaked to external host!")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer externalServer.Close()

	// Perform a request to an external URL using the client configured for api.trusted.com
	req, _ := http.NewRequest(http.MethodGet, externalServer.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
}

func TestShouldAuthenticateInvalidBaseURL(t *testing.T) {
	client := &Client{BaseURL: "http://a b.com"}
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if client.shouldAuthenticate(req) {
		t.Error("shouldAuthenticate should return false for invalid BaseURL")
	}
}
