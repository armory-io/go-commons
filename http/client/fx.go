package client

import (
	"github.com/armory-io/go-commons/oidc"
	"github.com/armory-io/go-commons/opentelemetry"
	"go.uber.org/fx"
	"net/http"
)

type authenticatedHTTPClientParameters struct {
	fx.In

	Identity *oidc.AccessTokenSupplier
	Tracing  opentelemetry.Configuration `optional:"true"`
}

var Module = fx.Module("armory-http",
	fx.Provide(func(params authenticatedHTTPClientParameters) *http.Client {
		return NewAuthenticatedHTTPClient(params.Identity, params.Tracing)
	}),
)
