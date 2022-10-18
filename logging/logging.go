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
	"os"
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
	loggerOptions := armoryStdLogOpt()

	switch strings.ToLower(appMd.LoggingType) {
	case "json":
		return createJSONLogger(appMd, loggerOptions)
	case "console":
		baseLogFields := getProductionLoggerFields(appMd)
		loggerOptions = append(loggerOptions, zap.Fields(baseLogFields...))
		return createArmoryConsoleLogger(loggerOptions, zapcore.InfoLevel)
	default:
		switch strings.ToLower(appMd.Environment) {
		case "production", "prod", "staging", "stage":
			return createJSONLogger(appMd, loggerOptions)
		default:
			return createArmoryConsoleLogger(loggerOptions, zapcore.InfoLevel)
		}
	}
}

func createJSONLogger(appMd metadata.ApplicationMetadata, loggerOptions []zap.Option) (*zap.Logger, error) {
	baseLogFields := getProductionLoggerFields(appMd)
	loggerOptions = append(loggerOptions, zap.Fields(baseLogFields...))
	return zap.NewProductionConfig().Build(loggerOptions...)
}

func getProductionLoggerFields(appMd metadata.ApplicationMetadata) []zap.Field {
	var baseLogFields []zap.Field
	baseLogFields = appendFieldIfPresent(applicationName, appMd.Name, baseLogFields)
	baseLogFields = appendFieldIfPresent(environment, appMd.Environment, baseLogFields)
	baseLogFields = appendFieldIfPresent(replicaSet, appMd.Replicaset, baseLogFields)
	baseLogFields = appendFieldIfPresent(hostname, appMd.Hostname, baseLogFields)
	baseLogFields = appendFieldIfPresent(version, appMd.Version, baseLogFields)
	return baseLogFields
}

func armoryStdLogOpt() []zap.Option {
	return []zap.Option{
		zap.WithCaller(true),
		// our internal error handling will add stack traces intelligently.
		zap.AddStacktrace(zap.DPanicLevel),
	}
}

func StdArmoryDevLogger(level zapcore.Level) (*zap.Logger, error) {
	return createArmoryConsoleLogger(armoryStdLogOpt(), level)
}

func createArmoryConsoleLogger(loggerOptions []zap.Option, level zapcore.Level) (*zap.Logger, error) {
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

	disableColors := false
	if os.Getenv("DISABLE_COLORS") == "true" {
		disableColors = true
	}

	return zap.New(
		zapcore.NewCore(NewArmoryDevConsoleEncoder(disableColors), sink, zap.NewAtomicLevelAt(level)),
		loggerOptions...,
	), nil
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
