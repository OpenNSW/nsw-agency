package httpclient

import (
	"net/http"

	"golang.org/x/oauth2/clientcredentials"
)

// OAuth2Authenticator implements the Client Credentials flow.
type OAuth2Authenticator struct {
	config *clientcredentials.Config
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
	token, err := o.config.TokenSource(req.Context()).Token()
	if err != nil {
		return err
	}
	token.SetAuthHeader(req)
	return nil
}
