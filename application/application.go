/*
 * Copyright (c) 2022 Armory, Inc
 *   National Electronics and Computer Technology Center, Thailand
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
