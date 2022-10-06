package server

import "github.com/armory-io/go-commons/http"

type Configuration struct {
	HTTP       http.HTTP
	Management http.HTTP
}
