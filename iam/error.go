package iam

import (
	"encoding/json"
	"net/http"
)

type defaultErr struct {
	Error string `json:"error"`
}

func errWriter(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(defaultErr{
		Error: msg,
	})
}
