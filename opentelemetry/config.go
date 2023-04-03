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

package opentelemetry

import (
	"context"
	"fmt"
	"github.com/armory-io/go-commons/metadata"
	"github.com/go-logr/zapr"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"time"
)

type (
	PushConfiguration struct {
		Enabled  bool
		Endpoint string
		APIKey   string
		Insecure bool
	}

	Configuration struct {
		SampleRate float64
		Push       PushConfiguration
	}
)

var (
	ErrInvalidConfiguration = errors.New("invalid tracing configuration")
)

func InitTracing(
	ctx context.Context,
	logger *zap.SugaredLogger,
	lc fx.Lifecycle,
	r *resource.Resource,
	config Configuration,
) error {
	if config.SampleRate < 0 || config.SampleRate > 1 {
		return fmt.Errorf("%w: sample rate must be between 0 and 1, got %f", ErrInvalidConfiguration, config.SampleRate)
	}

	tracingOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(config.SampleRate))),
		sdktrace.WithResource(r),
	}

	if config.Push.Enabled {
		logger.Info("Initializing tracing...")
		options := []otlptracehttp.Option{
			otlptracehttp.WithHeaders(map[string]string{
				"api-key": config.Push.APIKey,
			}),
			otlptracehttp.WithEndpoint(config.Push.Endpoint),
			otlptracehttp.WithURLPath("v1/traces"),
		}

		if config.Push.Insecure {
			options = append(options, otlptracehttp.WithInsecure())
		}

		client := otlptracehttp.NewClient(
			options...,
		)

		exporter, err := otlptrace.New(ctx, client)
		if err != nil {
			return err
		}
		tracingOpts = append(tracingOpts, sdktrace.WithBatcher(exporter))
	}

	tracerProvider := sdktrace.NewTracerProvider(tracingOpts...)
	otel.SetLogger(zapr.NewLogger(logger.Desugar()))
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return tracerProvider.Shutdown(ctx)
		},
	})

	return nil
}

func NewMeterProvider(
	ctx context.Context,
	r *resource.Resource,
	lc fx.Lifecycle,
	config Configuration,
) (*metric.MeterProvider, error) {
	var provider *metric.MeterProvider

	if config.Push.Enabled {
		exporter, err := otlpmetrichttp.New(
			ctx,
			otlpmetrichttp.WithEndpoint(config.Push.Endpoint),
			otlpmetrichttp.WithHeaders(map[string]string{
				"api-key": config.Push.APIKey,
			}),
		)
		if err != nil {
			return nil, err
		}
		provider = metric.NewMeterProvider(
			metric.WithReader(metric.NewPeriodicReader(exporter, metric.WithInterval(1*time.Second))),
			metric.WithResource(r),
		)
	} else {
		reader, err := otelprom.New(otelprom.WithRegisterer(prometheus.DefaultRegisterer))
		if err != nil {
			return nil, err
		}
		provider = metric.NewMeterProvider(metric.WithReader(reader))
	}

	global.SetMeterProvider(provider)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return provider.Shutdown(ctx)
		},
	})

	return provider, nil
}

func runtimeInstrumentation(
	mp *metric.MeterProvider,
	lc fx.Lifecycle,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return runtime.Start(
				runtime.WithMeterProvider(mp),
				runtime.WithMinimumReadMemStatsInterval(time.Second),
			)
		},
	})
}

func NewResource(
	ctx context.Context,
	app metadata.ApplicationMetadata,
) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(app.Name),
			semconv.ServiceVersionKey.String(app.Version),
			semconv.ServiceNamespaceKey.String("cdaas"),
			semconv.ServiceInstanceIDKey.String(app.Hostname),
			semconv.DeploymentEnvironmentKey.String(app.Environment),
			semconv.TelemetrySDKLanguageGo,
		),
	)
}

var Module = fx.Options(
	fx.Provide(NewResource),
	fx.Invoke(InitTracing),
	fx.Provide(NewMeterProvider),
	fx.Invoke(runtimeInstrumentation),
)
