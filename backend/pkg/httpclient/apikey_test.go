package httpclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAPIKeyAuthenticator(t *testing.T) {
	apiKey := "test-api-key"
	header := "X-Custom-Auth"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey := r.Header.Get(header)
		if gotKey != apiKey {
			t.Errorf("expected API key %q, got %q", apiKey, gotKey)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	auth := NewAPIKeyAuthenticator(apiKey, header)
	client := NewClient("", 5*time.Second, auth)

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.Status)
	}
}

func TestAPIKeyAuthenticatorDefaultHeader(t *testing.T) {
	apiKey := "test-api-key"
	defaultHeader := "X-API-Key"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey := r.Header.Get(defaultHeader)
		if gotKey != apiKey {
			t.Errorf("expected API key %q, got %q", apiKey, gotKey)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Pass empty header to trigger default
	auth := NewAPIKeyAuthenticator(apiKey, "")
	if auth.Header != defaultHeader {
		t.Errorf("expected default header %q, got %q", defaultHeader, auth.Header)
	}

	client := NewClient("", 5*time.Second, auth)

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.Status)
	}
}
