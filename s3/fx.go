package s3

import "go.uber.org/fx"

var Module = fx.Module("s3", fx.Provide(New))
