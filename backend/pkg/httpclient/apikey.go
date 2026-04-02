package httpclient

import "net/http"

// APIKeyAuthenticator is an authenticator that injects an API key into a header.
type APIKeyAuthenticator struct {
	APIKey string
	Header string
}

// NewAPIKeyAuthenticator creates a new APIKeyAuthenticator.
// If header is empty, it defaults to "X-API-Key".
func NewAPIKeyAuthenticator(apiKey, header string) *APIKeyAuthenticator {
	if header == "" {
		header = "X-API-Key"
	}
	return &APIKeyAuthenticator{
		APIKey: apiKey,
		Header: header,
	}
}

// Authenticate injects the API key into the specified header.
func (a *APIKeyAuthenticator) Authenticate(req *http.Request) error {
	req.Header.Set(a.Header, a.APIKey)
	return nil
}
