package auth

import (
	"encoding/base64"
	"net/http"
	"strings"
)

// Basic represents base64-encoded credentials for HTTP Basic Authentication.
type Basic struct {
	Username string
	Password string
}

// Authorization returns a function that yields the Proxy-Authorization header with Base64 encoded credentials.
func (a Basic) Authorization(req *http.Request) {
	req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte((a.Username+":"+a.Password))))
}

func ParseBasic(req *http.Request) (basic Basic, found bool) {
	auth := req.Header.Get("Proxy-Authorization")
	const prefix = "Basic "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}
	var username, password string
	if username, password, found = strings.Cut(string(c), ":"); found {
		basic.Username = username
		basic.Password = password
	}
	return
}
