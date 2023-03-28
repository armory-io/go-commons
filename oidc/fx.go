package oidc

import "go.uber.org/fx"

var Module = fx.Module("oidc", fx.Provide(
	NewAccessTokenSupplier,
))
