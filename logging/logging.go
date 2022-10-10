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
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"strings"
)

const (
	applicationName = "app"
	version         = "version"
	environment     = "environment"
	replicaSet      = "replicaset"
	hostname        = "hostname"
)

func ArmoryLoggerProvider(appMd metadata.ApplicationMetadata) (*zap.Logger, error) {
	var logger *zap.Logger

	loggerOptions := []zap.Option{
		zap.WithCaller(true),
		// our internal error handling will add stack traces intelligently.
		zap.AddStacktrace(zap.DPanicLevel),
	}

	switch strings.ToLower(appMd.Environment) {
	case "production", "prod", "staging", "stage":
		var baseLogFields []zap.Field
		baseLogFields = appendFieldIfPresent(applicationName, appMd.Name, baseLogFields)
		baseLogFields = appendFieldIfPresent(environment, appMd.Environment, baseLogFields)
		baseLogFields = appendFieldIfPresent(replicaSet, appMd.Replicaset, baseLogFields)
		baseLogFields = appendFieldIfPresent(hostname, appMd.Hostname, baseLogFields)
		baseLogFields = appendFieldIfPresent(version, appMd.Version, baseLogFields)

		loggerOptions = append(loggerOptions, zap.Fields(baseLogFields...))

		l, err := zap.NewProductionConfig().Build(loggerOptions...)
		if err != nil {
			return nil, err
		}
		logger = l
		break
	default:
		sink, closeOut, err := zap.Open("stderr")
		if err != nil {
			return nil, err
		}
		errSink, _, err := zap.Open("stderr")
		if err != nil {
			closeOut()
			return nil, err
		}

		loggerOptions = append(loggerOptions,
			zap.ErrorOutput(errSink),
			zap.Development(),
			zap.AddCaller(),
		)

		logger = zap.New(
			zapcore.NewCore(NewArmoryDevConsoleEncoder(false), sink, zap.NewAtomicLevelAt(zap.InfoLevel)),
			loggerOptions...,
		)
	}

	return logger, nil
}

func appendFieldIfPresent(key string, value string, fields []zap.Field) []zap.Field {
	if value != "" {
		return append(fields, zap.String(key, value))
	}
	return fields
}

var Module = fx.Options(
	fx.Provide(ArmoryLoggerProvider),
	fx.Provide(func(log *zap.Logger) *zap.SugaredLogger {
		return log.Sugar()
	}),
	fx.WithLogger(func(logger *zap.Logger) fxevent.Logger {
		return &fxevent.ZapLogger{Logger: logger}
	}),
)
