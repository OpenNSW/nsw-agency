package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestOAuth2Authenticator(t *testing.T) {
	clientID := "test-client-id"
	clientSecret := "test-client-secret"
	token := "test-bearer-token"

	// Mock token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST request to token URL, got %v", r.Method)
		}

		// In a real client credentials flow, we would check basic auth or form values
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token": "` + token + `", "token_type": "Bearer", "expires_in": 3600}`))
	}))
	defer tokenServer.Close()

	// Mock API server
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth := r.Header.Get("Authorization")
		expectedAuth := "Bearer " + token
		if gotAuth != expectedAuth {
			t.Errorf("expected Auth header %q, got %q", expectedAuth, gotAuth)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	auth := NewOAuth2Authenticator(clientID, clientSecret, tokenServer.URL, nil)
	client := NewClientBuilder().
		WithTimeout(5 * time.Second).
		WithAuthenticator(auth).
		Build()

	resp, err := client.Get(apiServer.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.Status)
	}
}

func TestOAuth2AuthenticatorFailure(t *testing.T) {
	// Mock token server that returns an error
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer tokenServer.Close()

	auth := NewOAuth2Authenticator("id", "secret", tokenServer.URL, nil)
	client := NewClientBuilder().
		WithTimeout(5 * time.Second).
		WithAuthenticator(auth).
		Build()

	_, err := client.Get("http://example.com")
	if err == nil {
		t.Error("expected error on token fetch failure")
	}
}

func TestOAuth2AuthenticatorWithInsecureTLS(t *testing.T) {
	token := "tls-bearer-token"

	// Self-signed cert token server — would fail without InsecureSkipVerify
	tokenServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token": "%s", "token_type": "Bearer", "expires_in": 3600}`, token)
	}))
	defer tokenServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	auth := NewOAuth2Authenticator("id", "secret", tokenServer.URL+"/token", nil)
	client := NewClientBuilder().
		WithTimeout(5 * time.Second).
		WithTLS(&TLSConfig{InsecureSkipVerify: true}).
		WithAuthenticator(auth).
		Build()

	resp, err := client.Get(apiServer.URL)
	if err != nil {
		t.Fatalf("token fetch failed TLS verification: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %v", resp.Status)
	}
}

// TestOAuth2AuthenticatorCachesToken verifies the access token is fetched once
// and reused across many requests, rather than re-fetched on every request.
func TestOAuth2AuthenticatorCachesToken(t *testing.T) {
	var tokenHits int32

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&tokenHits, 1)
		w.Header().Set("Content-Type", "application/json")
		// Long-lived token so it stays valid for the whole test.
		fmt.Fprint(w, `{"access_token": "cached-token", "token_type": "Bearer", "expires_in": 3600}`)
	}))
	defer tokenServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer cached-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	auth := NewOAuth2Authenticator("id", "secret", tokenServer.URL, nil)
	client := NewClientBuilder().
		WithTimeout(5 * time.Second).
		WithAuthenticator(auth).
		Build()

	for i := 0; i < 5; i++ {
		resp, err := client.Get(apiServer.URL)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: expected 200 OK, got %v", i, resp.Status)
		}
	}

	if got := atomic.LoadInt32(&tokenHits); got != 1 {
		t.Errorf("expected token endpoint to be hit once, got %d", got)
	}
}

// TestOAuth2AuthenticatorRefreshesExpiredToken verifies that a token within the
// reuse expiry buffer is treated as expired and a fresh one is fetched.
func TestOAuth2AuthenticatorRefreshesExpiredToken(t *testing.T) {
	var tokenHits int32

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&tokenHits, 1)
		w.Header().Set("Content-Type", "application/json")
		// expires_in below the 10s expiryDelta => each issued token is already
		// "expired" from ReuseTokenSource's perspective, forcing a refresh on
		// the next request. The bearer value changes per fetch so we can assert
		// the API received the most recent token.
		fmt.Fprintf(w, `{"access_token": "token-%d", "token_type": "Bearer", "expires_in": 1}`, n)
	}))
	defer tokenServer.Close()

	var lastBearer atomic.Value
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastBearer.Store(r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	auth := NewOAuth2Authenticator("id", "secret", tokenServer.URL, nil)
	client := NewClientBuilder().
		WithTimeout(5 * time.Second).
		WithAuthenticator(auth).
		Build()

	resp1, err := client.Get(apiServer.URL)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	resp1.Body.Close()
	if got := lastBearer.Load(); got != "Bearer token-1" {
		t.Errorf("first request: expected %q, got %q", "Bearer token-1", got)
	}

	resp2, err := client.Get(apiServer.URL)
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	resp2.Body.Close()
	if got := lastBearer.Load(); got != "Bearer token-2" {
		t.Errorf("second request: expected refreshed %q, got %q", "Bearer token-2", got)
	}

	if got := atomic.LoadInt32(&tokenHits); got != 2 {
		t.Errorf("expected token endpoint to be hit twice (refresh), got %d", got)
	}
}

// TestOAuth2AuthenticatorHonoursRequestContext verifies the token fetch respects
// the cancellation/deadline of the request's context. The client is built WITHOUT
// a timeout, so only the request context can abort a hung token endpoint — proving
// the fetch is not bound to a long-lived background context.
func TestOAuth2AuthenticatorHonoursRequestContext(t *testing.T) {
	// Token server that blocks until either its request context is cancelled or
	// the test releases it. release is closed before tokenServer.Close() (defers
	// run LIFO) so teardown never waits on a stuck handler.
	release := make(chan struct{})
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer tokenServer.Close()
	defer close(release)

	auth := NewOAuth2Authenticator("id", "secret", tokenServer.URL, nil)
	client := NewClientBuilder().
		WithAuthenticator(auth).
		Build()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	start := time.Now()
	_, err = client.Do(req)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from a token fetch cancelled by the request context, got nil")
	}
	if elapsed > 2*time.Second {
		t.Errorf("token fetch did not honour the request context; took %v", elapsed)
	}
}
