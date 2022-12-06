package yeti

import "go.uber.org/fx"

var Module = fx.Module("yeti", fx.Provide(NewClient))
