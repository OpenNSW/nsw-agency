package httpclient

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a wrapper around *http.Client with built-in authentication and a base URL.
type Client struct {
	httpClient *http.Client
	auth       Authenticator
	BaseURL    string
}

// NewClient creates a new HTTP client with the given authenticator and base URL.
func NewClient(baseURL string, timeout time.Duration, auth Authenticator) *Client {
	if baseURL != "" && !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		auth:    auth,
		BaseURL: baseURL,
	}
}

// Do performs an HTTP request and applies authentication.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c.auth != nil && c.shouldAuthenticate(req) {
		if err := c.auth.Authenticate(req); err != nil {
			return nil, err
		}
	}
	return c.httpClient.Do(req)
}

// shouldAuthenticate checks if the request URL aligns with the BaseURL.
func (c *Client) shouldAuthenticate(req *http.Request) bool {
	if c.BaseURL == "" {
		return true
	}
	baseURL, err := url.Parse(c.BaseURL)
	if err != nil {
		return false
	}
	return req.URL.Host == baseURL.Host && req.URL.Scheme == baseURL.Scheme
}

// resolveURL joins the base URL with the provided path.
func (c *Client) resolveURL(path string) (string, error) {
	if c.BaseURL == "" {
		return path, nil
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", err
	}
	// If path is already an absolute URL, use it directly
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path, nil
	}
	rel, err := url.Parse(strings.TrimPrefix(path, "/"))
	if err != nil {
		return "", err
	}
	return base.ResolveReference(rel).String(), nil
}

// Get performs a GET request relative to the BaseURL.
func (c *Client) Get(path string) (*http.Response, error) {
	fullURL, err := c.resolveURL(path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Post performs a POST request relative to the BaseURL.
func (c *Client) Post(path string, contentType string, body []byte) (*http.Response, error) {
	fullURL, err := c.resolveURL(path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.Do(req)
}
