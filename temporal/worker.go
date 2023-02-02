package temporal

import (
	"github.com/armory-io/go-commons/tracing"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
)

type WorkerProvider struct {
	tracing tracing.Configuration
}

func WorkerProviderProvider(tracingConfig tracing.Configuration) WorkerProvider {
	return WorkerProvider{tracing: tracingConfig}
}

func (w *WorkerProvider) NewWorker(c client.Client, taskQueue string) (worker.Worker, error) {
	interceptors := []interceptor.WorkerInterceptor{newWorkflowContextInterceptor()}

	// The no-op trace provider causes the interceptor to crash, which
	// is why this package knows about the tracing config :(
	if w.tracing.Push.Enabled {
		otelInterceptor, err := newOtelInterceptor()
		if err != nil {
			return nil, err
		}
		interceptors = append(interceptors, otelInterceptor)
	}

	return worker.New(c, taskQueue, worker.Options{
		Interceptors: interceptors,
	}), nil
}
