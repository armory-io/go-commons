package mysql

import "go.uber.org/fx"

var Module = fx.Module(
	"sql",
	fx.Provide(New),
	fx.Invoke(NewMigrator),
)
