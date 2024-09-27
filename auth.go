package httpproxy

import (
	"encoding/base64"
	"iter"
)

// BasicAuthentication represents base64-encoded credentials for HTTP Basic Authentication.
type BasicAuthentication struct {
	Username string
	Password string
}

// Header returns a function that yields the Proxy-Authorization header with Base64 encoded credentials.
func (a *BasicAuthentication) Header() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		yield("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte((a.Username+":"+a.Password))))
	}
}
