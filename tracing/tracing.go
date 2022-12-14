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

package tracing

import (
	"context"
	"github.com/armory-io/go-commons/metadata"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type PushConfiguration struct {
	Enabled  bool
	Endpoint string
	APIKey   string
}

type Configuration struct {
	Push PushConfiguration
}

func InitTracing(
	ctx context.Context,
	logger *zap.SugaredLogger,
	lc fx.Lifecycle,
	app metadata.ApplicationMetadata,
	config Configuration,
) error {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(app.Name),
			semconv.ServiceVersionKey.String(app.Version),
			semconv.ServiceNamespaceKey.String(app.Replicaset),
			semconv.ServiceInstanceIDKey.String(app.Hostname),
			semconv.DeploymentEnvironmentKey.String(app.Environment),
		),
	)
	if err != nil {
		return err
	}

	tracingOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
	}

	if config.Push.Enabled {
		logger.Info("Initializing OTEL tracing...")
		client := otlptracehttp.NewClient(
			otlptracehttp.WithHeaders(map[string]string{
				"api-key": config.Push.APIKey,
			}),
			otlptracehttp.WithEndpoint(config.Push.Endpoint),
			otlptracehttp.WithURLPath("v1/traces"),
		)

		exporter, err := otlptrace.New(ctx, client)
		if err != nil {
			return err
		}
		tracingOpts = append(tracingOpts, sdktrace.WithBatcher(exporter))
	}

	tracerProvider := sdktrace.NewTracerProvider(tracingOpts...)

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return tracerProvider.Shutdown(ctx)
		},
	})

	return nil
}

var Module = fx.Options(
	fx.Invoke(InitTracing),
)
