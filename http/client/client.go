package client

import (
	"github.com/armory-io/go-commons/iam/token"
	"github.com/armory-io/go-commons/opentelemetry"
	"github.com/hashicorp/go-cleanhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net/http"
)

type (
	Parameters struct {
		Tracing opentelemetry.Configuration `optional:"true"`
	}

	AuthenticatedClientParameters struct {
		fx.In

		Log      *zap.SugaredLogger
		Identity token.Identity
		Tracing  opentelemetry.Configuration `optional:"true"`
	}
)

// NewRoundTripper creates an http.RoundTripper that propagates OpenTelemetry trace headers.
func NewRoundTripper(params Parameters) (http.RoundTripper, error) {
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
		), nil
	}

	return base, nil
}

// NewHTTPClient creates an http.Client that propagates OpenTelemetry trace headers.
func NewHTTPClient(params Parameters) (*http.Client, error) {
	rt, err := NewRoundTripper(params)
	if err != nil {
		return nil, err
	}

	c := cleanhttp.DefaultClient()
	c.Transport = rt

	return c, nil
}

// NewAuthenticatedHTTPClient creates an http.Client that propagates OpenTelemetry trace headers and authenticates its requests
// with a bearer token header.
func NewAuthenticatedHTTPClient(params AuthenticatedClientParameters) (*http.Client, error) {
	c, err := NewHTTPClient(Parameters{Tracing: params.Tracing})
	if err != nil {
		return nil, err
	}

	c.Transport = token.GetTokenWrapper(c.Transport, params.Identity, params.Log)
	return c, nil
}
