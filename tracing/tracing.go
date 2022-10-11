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
)

type NewRelicConfiguration struct {
	Enabled bool
	APIKey  string
}

type Configuration struct {
	NewRelic NewRelicConfiguration
}

func InitTracing(
	ctx context.Context,
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

	tracingOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
	}

	var exporter *otlptrace.Exporter
	if config.NewRelic.Enabled {
		client := otlptracehttp.NewClient(
			otlptracehttp.WithHeaders(map[string]string{
				"api-key": config.NewRelic.APIKey,
			}),
			otlptracehttp.WithEndpoint("otlp.nr-data.net:4318"),
			otlptracehttp.WithURLPath("v1/traces"),
		)
		exporter, err = otlptrace.New(ctx, client)
		if err != nil {
			return err
		}
		tracingOpts = append(tracingOpts, sdktrace.WithBatcher(exporter))
	}

	tracerProvider := sdktrace.NewTracerProvider(tracingOpts...)

	if err != nil {
		return err
	}
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
