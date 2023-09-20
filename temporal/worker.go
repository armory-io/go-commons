package temporal

import (
	"github.com/armory-io/go-commons/opentelemetry"
	"github.com/samber/lo"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

type WorkerProvider struct {
	tracing opentelemetry.Configuration
}

func WorkerProviderProvider(tracingConfig opentelemetry.Configuration) WorkerProvider {
	return WorkerProvider{tracing: tracingConfig}
}

func (w *WorkerProvider) NewWorker(c client.Client, taskQueue string) (worker.Worker, error) {
	return w.NewWorkerWithOptions(c, taskQueue, &worker.Options{})
}

func (w *WorkerProvider) NewWorkerWithOptions(c client.Client, taskQueue string, options *worker.Options) (worker.Worker, error) {
	return w.newWorker(c, taskQueue, lo.Ternary(options == nil, &worker.Options{}, options))
}

func (w *WorkerProvider) newWorker(c client.Client, taskQueue string, opts *worker.Options) (worker.Worker, error) {
	opts.Interceptors = append(opts.Interceptors, newWorkflowContextInterceptor())

	// The no-op trace provider causes the interceptor to crash, which
	// is why this package knows about the tracing config :(
	if w.tracing.Push.Enabled {
		otelInterceptor, err := newOtelInterceptor()
		if err != nil {
			return nil, err
		}
		opts.Interceptors = append(opts.Interceptors, otelInterceptor)
	}

	return worker.New(c, taskQueue, *opts), nil
}
