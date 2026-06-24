package httpclient

import (
	"context"
	"net/http"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// OAuth2Authenticator implements the Client Credentials flow.
type OAuth2Authenticator struct {
	config *clientcredentials.Config

	// tokenSource is built once and reused so the underlying ReuseTokenSource
	// caches the access token and only re-fetches when it nears expiry.
	once        sync.Once
	tokenSource oauth2.TokenSource
}

// NewOAuth2Authenticator creates a new OAuth2Authenticator.
func NewOAuth2Authenticator(clientID, clientSecret, tokenURL string, scopes []string) *OAuth2Authenticator {
	return &OAuth2Authenticator{
		config: &clientcredentials.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			TokenURL:     tokenURL,
			Scopes:       scopes,
		},
	}
}

// Authenticate fetches a token if necessary and injects it into the request header.
func (o *OAuth2Authenticator) Authenticate(req *http.Request) error {
	o.once.Do(func() {
		// Build the token source once and bind it to a long-lived context so
		// refreshes survive the cancellation of any individual request. Carry
		// over the *http.Client that Client.Do injects (via oauth2.HTTPClient)
		// so token fetches use the same, possibly InsecureSkipVerify, transport.
		ctx := context.Background()
		if hc, ok := req.Context().Value(oauth2.HTTPClient).(*http.Client); ok {
			ctx = context.WithValue(ctx, oauth2.HTTPClient, hc)
		}
		o.tokenSource = o.config.TokenSource(ctx)
	})

	token, err := o.tokenSource.Token()
	if err != nil {
		return err
	}
	token.SetAuthHeader(req)
	return nil
}
