package core

import (
	"github.com/armory-io/go-commons/opentelemetry"
	"github.com/hashicorp/go-cleanhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
	"net/http"
)

type (
	Parameters struct {
		Tracing opentelemetry.Configuration `optional:"true"`
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
