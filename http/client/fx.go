package client

import (
	"go.uber.org/fx"
)

var Module = fx.Module("armory-http",
	fx.Provide(NewAuthenticatedHTTPClient),
)
