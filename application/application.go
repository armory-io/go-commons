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

package application

import (
	"github.com/armory-io/go-commons/gin"
	armoryhttp "github.com/armory-io/go-commons/http"
	"github.com/armory-io/go-commons/iam"
	"github.com/armory-io/go-commons/logging"
	"github.com/armory-io/go-commons/metadata"
	"github.com/armory-io/go-commons/metrics"
	"github.com/armory-io/go-commons/mysql"
	"go.uber.org/fx"
)

// Configuration defines required settings for the application module.
type Configuration struct {
	fx.Out

	Server   armoryhttp.Configuration
	Metrics  metrics.Configuration
	Auth     iam.Configuration
	Database mysql.Configuration
}

var Module = fx.Module("armory-application",
	logging.Module,
	metadata.Module,
	fx.Provide(metrics.New),
	fx.Provide(iam.New),
	fx.Provide(gin.NewGinServer),
)
