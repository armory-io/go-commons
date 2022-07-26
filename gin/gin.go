/*
 * Copyright 2022 Armory, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gin

import (
	"context"
	armoryhttp "github.com/armory-io/go-commons/http"
	"github.com/armory-io/go-commons/iam"
	"github.com/armory-io/go-commons/metrics"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net/http"
)

type AllowWithoutAuth struct {
	fx.Out

	Routes []string `group:"allow-without-auth"`
}

type GinServerParams struct {
	fx.In

	Allowed [][]string `group:"allow-without-auth"`
}

// NewGinServer Creates and exports a Gin Server
// Deprecated: this package will be deleted once Yeti can be ported to use controllers and the Start Sever Hook
func NewGinServer(
	lifecycle fx.Lifecycle,
	config armoryhttp.Configuration,
	logger *zap.SugaredLogger,
	ps *iam.ArmoryCloudPrincipalService,
	ms metrics.MetricsSvc,
	gsp GinServerParams,
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
	g.Use(metrics.GinHTTPMiddleware(ms))
	g.Use(iam.GinAuthMiddleware(ps, lo.Flatten(gsp.Allowed)))

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
