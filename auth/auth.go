package auth

import "net/http"

type Authorization interface {
	Authorization(*http.Request)
}
