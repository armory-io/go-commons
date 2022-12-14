package temporal

import (
	temporalotel "go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"
)

func newOtelInterceptor() (interceptor.Interceptor, error) {
	// Uses global tracer provider.
	return temporalotel.NewTracingInterceptor(temporalotel.TracerOptions{})
}
