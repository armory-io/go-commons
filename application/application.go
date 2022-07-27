package application

import (
	"github.com/armory-io/go-commons/gin"
	armoryhttp "github.com/armory-io/go-commons/http"
	"github.com/armory-io/go-commons/iam"
	"github.com/armory-io/go-commons/logging"
	"github.com/armory-io/go-commons/metrics"
	"github.com/armory-io/go-commons/mysql"
	"go.uber.org/fx"
)

// Settings defines required settings for the application module.
type Settings struct {
	fx.Out

	Logging  logging.Settings
	Server   armoryhttp.ServerSettings
	Metrics  metrics.Settings
	Auth     iam.Settings
	Database mysql.Settings
}

var Module = fx.Module("armory-application",
	fx.Provide(logging.New),
	fx.Provide(metrics.New),
	fx.Provide(iam.New),
	fx.Provide(gin.NewGinServer),
)
