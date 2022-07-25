package gin

import (
	"context"
	armoryhttp "github.com/armory-io/lib-go-armory-cloud-commons/http"
	"github.com/armory-io/lib-go-armory-cloud-commons/iam"
	"github.com/armory-io/lib-go-armory-cloud-commons/metrics"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net/http"
)

func NewGinServer(
	lifecycle fx.Lifecycle,
	config armoryhttp.ServerSettings,
	logger *zap.SugaredLogger,
	ps *iam.ArmoryCloudPrincipalService,
	ms *metrics.Metrics,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()

	g.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	// Ideally the middleware would be decoupled from one another
	// but we need to make sure the middleware are applied in order.
	g.Use(armoryhttp.GinClientVersionMiddleware)
	g.Use(metrics.GinHTTPMiddleware(ms))
	g.Use(iam.GinAuthMiddleware(ps))

	server := armoryhttp.NewServer(config)

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := server.Start(g); err != nil {
					logger.Errorf("Failed to start server: %s", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})

	return g
}
