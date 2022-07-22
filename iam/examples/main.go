package main

import (
	"errors"
	"fmt"
	"github.com/armory-io/lib-go-armory-cloud-commons/iam"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

// JWK url (set in config)
const keysURL = "http://armory.io"

func PathHandler(response http.ResponseWriter, request *http.Request) {
	fmt.Fprintf(response, "Hi!")
}

func myCustomValidator(p *iam.ArmoryCloudPrincipal) error {
	if p.HasScope("read:me") {
		return nil
	}
	return errors.New("sorry you are not allowed")
}

func main() {
	// Instantiating principal service
	ps, err := iam.CreatePrincipalServiceInstance(keysURL, myCustomValidator)
	if err != nil {
		log.Fatal(err, "failed to initialize principal service")
	}

	// Registering handlers
	r := mux.NewRouter()
	r.HandleFunc("/", PathHandler)
	http.Handle("/", ps.ArmoryCloudPrincipalMiddleware(r))
}
