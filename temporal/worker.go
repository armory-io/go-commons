package temporal

import (
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
)

func NewWorker(c client.Client, taskQueue string) (worker.Worker, error) {
	otelInterceptor, err := newOtelInterceptor()
	if err != nil {
		return nil, err
	}

	return worker.New(c, taskQueue, worker.Options{
		Interceptors: []interceptor.WorkerInterceptor{NewLoggerInterceptor(), otelInterceptor},
	}), nil
}
