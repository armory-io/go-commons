package client

import (
	"fmt"
	"github.com/armory-io/go-commons/oidc"
	"github.com/armory-io/go-commons/opentelemetry"
	"github.com/hashicorp/go-cleanhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/fx"
	"net/http"
)

type (
	Parameters struct {
		Tracing opentelemetry.Configuration `optional:"true"`
	}

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
		GetToken() (string, error)
	}
)

// NewRoundTripper creates an http.RoundTripper that propagates OpenTelemetry trace headers.
func NewRoundTripper(params Parameters) http.RoundTripper {
	base := cleanhttp.DefaultTransport()

	if params.Tracing.Push.Enabled {
		return otelhttp.NewTransport(
			base,
			otelhttp.WithPropagators(
				propagation.NewCompositeTextMapPropagator(
					propagation.TraceContext{},
					propagation.Baggage{},
				),
			),
		)
	}

	return base
}

// NewHTTPClient creates an http.Client that propagates OpenTelemetry trace headers.
func NewHTTPClient(params Parameters) *http.Client {
	rt := NewRoundTripper(params)
	c := cleanhttp.DefaultClient()
	c.Transport = rt
	return c
}

// NewAuthenticatedHTTPClient creates an http.Client that propagates OpenTelemetry trace headers and authenticates its requests
// with a bearer token header.
func NewAuthenticatedHTTPClient(params AuthenticatedClientParameters) *http.Client {
	c := NewHTTPClient(Parameters{Tracing: params.Tracing})

	c.Transport = &bearerTokenRoundTripper{
		tokenSupplier: params.Identity,
		base:          c.Transport,
	}
	return c
}

func (b *bearerTokenRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	token, err := b.tokenSupplier.GetToken()
	if err != nil {
		return nil, err
	}

	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	return b.base.RoundTrip(request)
}
