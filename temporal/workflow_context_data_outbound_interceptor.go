package temporal

import (
	"context"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

type workflowContextDataInterceptor struct {
	interceptor.WorkerInterceptorBase
}

func newWorkflowContextInterceptor() interceptor.WorkerInterceptor {
	return &workflowContextDataInterceptor{}
}

func (w *workflowContextDataInterceptor) InterceptActivity(
	ctx context.Context,
	next interceptor.ActivityInboundInterceptor,
) interceptor.ActivityInboundInterceptor {
	i := &activityInboundLoggerInterceptor{root: w}
	i.Next = next
	return i
}

type activityInboundLoggerInterceptor struct {
	interceptor.ActivityInboundInterceptorBase
	root *workflowContextDataInterceptor
}

func (a *activityInboundLoggerInterceptor) Init(outbound interceptor.ActivityOutboundInterceptor) error {
	i := &activityOutboundLoggerInterceptor{root: a.root}
	i.Next = outbound
	return a.Next.Init(i)
}

type activityOutboundLoggerInterceptor struct {
	interceptor.ActivityOutboundInterceptorBase
	root *workflowContextDataInterceptor
}

func (a *activityOutboundLoggerInterceptor) GetLogger(ctx context.Context) log.Logger {
	logger := a.Next.GetLogger(ctx)
	return withFields(logger, getFields(ctx))
}

func (w *workflowContextDataInterceptor) InterceptWorkflow(
	ctx workflow.Context,
	next interceptor.WorkflowInboundInterceptor,
) interceptor.WorkflowInboundInterceptor {
	i := &workflowContextInboundInterceptor{root: w}
	i.Next = next
	return i
}

type workflowContextInboundInterceptor struct {
	interceptor.WorkflowInboundInterceptorBase
	root *workflowContextDataInterceptor
}

func (w *workflowContextInboundInterceptor) Init(outbound interceptor.WorkflowOutboundInterceptor) error {
	i := &workflowOutboundLoggerInterceptor{root: w.root}
	i.Next = outbound
	return w.Next.Init(i)
}

type workflowOutboundLoggerInterceptor struct {
	interceptor.WorkflowOutboundInterceptorBase
	root *workflowContextDataInterceptor
}

func (w *workflowOutboundLoggerInterceptor) GetLogger(ctx workflow.Context) log.Logger {
	logger := w.Next.GetLogger(ctx)
	return withFields(logger, getFields(ctx))
}

func (w *workflowOutboundLoggerInterceptor) GetMetricsHandler(ctx workflow.Context) client.MetricsHandler {
	handler := w.Next.GetMetricsHandler(ctx)
	return withTags(handler, getTagsFromWorkflowContext(ctx))
}
