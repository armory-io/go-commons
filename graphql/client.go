package graphql

import (
	"fmt"
	"github.com/armory-io/go-commons/iam"
	"github.com/hasura/go-graphql-client"
	"github.com/pkg/errors"
	"net/http"
)

var (
	ErrUserPrincipalNotFound = errors.New("could not find user principal in request context")
)

const (
	adminSecretHeader = "x-hasura-admin-secret"
	orgIDHeader       = "x-hasura-org-id"
	envIDHeader       = "x-hasura-env-id"
	roleHeader        = "x-hasura-role"
	superuserRole     = "armory:hasura:admin"
)

type Configuration struct {
	BaseURL string

	// If provided, the client will add the Hasura server admin secret as a request header.
	// This is not advised for production, since it is not easy to rotate Hasura admin secrets.
	AdminSecret string
}

// NewClient returns a new GraphQL client.
// The provided HTTP client should supply its own bearer token.
// You can use this client to make requests on behalf of tenants in the following way:
// - The HTTP client's bearer token should have an "admin" scope (OR, in development or staging, you can set the Hasura admin secret).
// - This GraphQL client will assume the "armory:hasura:admin" role (only possible because of the "admin" scope).
// - When making requests with the GraphQL client, pass a context with an iam.ArmoryCloudPrincipal.
func NewClient(config Configuration, hc *http.Client) *graphql.Client {
	c := graphql.NewClient(config.BaseURL, &doer{
		client: hc,
	})

	return c.WithRequestModifier(func(request *http.Request) {
		if config.AdminSecret != "" {
			request.Header.Add(adminSecretHeader, config.AdminSecret)
		}

		principal, err := iam.ExtractPrincipalFromContext(request.Context())
		if err != nil || principal == nil || principal.OrgId == "" || principal.EnvId == "" {
			// We should not let the request go out if the principal isn't present,
			// because Hasura will store (or query) data under the requesting token's tenant (an Armory internal tenant).
			// We'll be able to return a real error later on in the request lifecycle.
			return
		}

		request.Header.Add(orgIDHeader, principal.OrgId)
		request.Header.Add(envIDHeader, principal.EnvId)
		request.Header.Add(roleHeader, superuserRole)
	})
}

type doer struct {
	client *http.Client
}

func (d *doer) Do(request *http.Request) (*http.Response, error) {
	principal, err := iam.ExtractPrincipalFromContext(request.Context())
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUserPrincipalNotFound, err)
	} else if principal == nil || principal.OrgId == "" || principal.EnvId == "" {
		return nil, ErrUserPrincipalNotFound
	}
	return d.client.Do(request)
}
