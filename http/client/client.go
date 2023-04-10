package client

import (
	"context"
	"fmt"
	"github.com/armory-io/go-commons/http/client/core"
	"github.com/armory-io/go-commons/oidc"
	"github.com/armory-io/go-commons/opentelemetry"
	"go.uber.org/fx"
	"net/http"
)

type (
	AuthenticatedClientParameters struct {
		fx.In

		Identity *oidc.AccessTokenSupplier
		Tracing  opentelemetry.Configuration `optional:"true"`
	}

	bearerTokenRoundTripper struct {
		base          http.RoundTripper
		tokenSupplier tokenSupplier
	}

	tokenSupplier interface {
		GetToken(ctx context.Context) (string, error)
	}
)

// NewAuthenticatedHTTPClient creates an http.Client that propagates OpenTelemetry trace headers and authenticates its requests
// with a bearer token header.
func NewAuthenticatedHTTPClient(params AuthenticatedClientParameters) *http.Client {
	c := core.NewHTTPClient(core.Parameters{Tracing: params.Tracing})

	c.Transport = &bearerTokenRoundTripper{
		tokenSupplier: params.Identity,
		base:          c.Transport,
	}
	return c
}

func (b *bearerTokenRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	token, err := b.tokenSupplier.GetToken(request.Context())
	if err != nil {
		return nil, err
	}

	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	return b.base.RoundTrip(request)
}
