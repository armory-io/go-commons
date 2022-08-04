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

package logging

import (
	"github.com/armory-io/go-commons/metadata"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"strings"
)

const (
	applicationName = "app"
	version         = "version"
	environment     = "environment"
	replicaSet      = "replicaset"
	hostname        = "hostname"
)

func ArmoryLoggerProvider(appMd metadata.ApplicationMetadata) (*zap.SugaredLogger, error) {
	var logger *zap.Logger

	var baseLogFields []zap.Field
	baseLogFields = appendFieldIfPresent(applicationName, appMd.Name, baseLogFields)
	baseLogFields = appendFieldIfPresent(environment, appMd.Environment, baseLogFields)
	baseLogFields = appendFieldIfPresent(replicaSet, appMd.Replicaset, baseLogFields)
	baseLogFields = appendFieldIfPresent(hostname, appMd.Hostname, baseLogFields)
	baseLogFields = appendFieldIfPresent(version, appMd.Version, baseLogFields)

	loggerOptions := []zap.Option{
		zap.WithCaller(true),
		zap.Fields(baseLogFields...),
	}

	switch strings.ToLower(appMd.Environment) {
	case "production", "prod", "staging", "stage":
		l, err := zap.NewProductionConfig().Build(loggerOptions...)
		if err != nil {
			return nil, err
		}
		logger = l
	default:
		l, err := zap.NewDevelopment(loggerOptions...)
		if err != nil {
			return nil, err
		}
		logger = l
	}

	return logger.Sugar(), nil
}

func appendFieldIfPresent(key string, value string, fields []zap.Field) []zap.Field {
	if value != "" {
		return append(fields, zap.String(key, value))
	}
	return fields
}

var Module = fx.Options(
	fx.Provide(ArmoryLoggerProvider),
)
