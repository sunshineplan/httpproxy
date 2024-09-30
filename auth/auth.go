package auth

import "net/http"

// Authorization is an interface for types that can set authorization headers
// in HTTP requests.
type Authorization interface {
	// Authorization sets the appropriate authorization header in the given
	// HTTP request.
	Authorization(*http.Request)
}
