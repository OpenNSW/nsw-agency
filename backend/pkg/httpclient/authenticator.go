package httpclient

import "net/http"

// Authenticator is an interface for applying authentication to an HTTP request.
type Authenticator interface {
	Authenticate(req *http.Request) error
}
