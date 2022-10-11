package temporal

import (
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
)

func NewWorker(c client.Client, taskQueue string) worker.Worker {
	return worker.New(c, taskQueue, worker.Options{
		Interceptors: []interceptor.WorkerInterceptor{NewLoggerInterceptor()},
	})
}
