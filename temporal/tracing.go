package temporal

import (
	"context"
	"github.com/samber/lo"
	temporalotel "go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/workflow"
	"strconv"
)

type (
	workflowTraceParametersKey        struct{}
	workflowTraceParametersPropagator struct{}

	TaskResult[T any] struct {
		Result   T
		Err      error
		Attempts int
		Status   string
	}
)

const tracingKey = "armory-tracing"

func (w workflowTraceParametersPropagator) Inject(ctx context.Context, writer workflow.HeaderWriter) error {
	kvp, ok := ctx.Value(workflowTraceParametersKey{}).(*map[string]string)
	if ok && nil != kvp {
		payload, err := converter.GetDefaultDataConverter().ToPayload(*kvp)
		if err != nil {
			return err
		}
		writer.Set(tracingKey, payload)
	}
	return nil
}

func (w workflowTraceParametersPropagator) Extract(ctx context.Context, reader workflow.HeaderReader) (context.Context, error) {
	if raw, ok := reader.Get(tracingKey); ok {
		var kvp map[string]string
		if err := converter.GetDefaultDataConverter().FromPayload(raw, &kvp); err != nil {
			return ctx, nil
		}
		ctx = context.WithValue(ctx, workflowTraceParametersKey{}, &kvp)
	}
	return ctx, nil
}

func (w workflowTraceParametersPropagator) InjectFromWorkflow(ctx workflow.Context, writer workflow.HeaderWriter) error {
	kvp, ok := ctx.Value(workflowTraceParametersKey{}).(*map[string]string)
	if ok {
		payload, err := converter.GetDefaultDataConverter().ToPayload(*kvp)
		if err != nil {
			return err
		}
		writer.Set(tracingKey, payload)
	}
	return nil
}

func (w workflowTraceParametersPropagator) ExtractToWorkflow(ctx workflow.Context, reader workflow.HeaderReader) (workflow.Context, error) {
	if raw, ok := reader.Get(tracingKey); ok {
		var kvps map[string]string
		if err := converter.GetDefaultDataConverter().FromPayload(raw, &kvps); err != nil {
			return ctx, nil
		}
		ctx = workflow.WithValue(ctx, workflowTraceParametersKey{}, &kvps)
	}
	return ctx, nil
}

func newOtelInterceptor() (interceptor.Interceptor, error) {
	// Uses global tracer provider.
	return temporalotel.NewTracingInterceptor(temporalotel.TracerOptions{})
}

func NewWorkflowTraceParametersPropagator() workflow.ContextPropagator {
	return &workflowTraceParametersPropagator{}
}

func WithTraceParameters(ctx context.Context, entries ...string) context.Context {
	container := getOrCreateTags(ctx.Value)
	return context.WithValue(ctx, workflowTraceParametersKey{}, makeTraceabilityTags(container, entries))
}

func WithWorkflowTraceParameters(ctx workflow.Context, entries ...string) workflow.Context {
	container := getOrCreateTags(ctx.Value)
	return workflow.WithValue(ctx, workflowTraceParametersKey{}, makeTraceabilityTags(container, entries))
}

func getOrCreateTags(contextGetter func(key any) any) *map[string]string {
	existing, ok := contextGetter(workflowTraceParametersKey{}).(*map[string]string)
	return lo.IfF(ok && existing != nil, func() *map[string]string {
		copy := map[string]string{}
		for k, v := range *existing {
			copy[k] = v
		}
		return &copy
	}).ElseF(func() *map[string]string {
		return &map[string]string{}
	})
}

func makeTraceabilityTags(tags *map[string]string, entries []string) *map[string]string {
	if len(entries)%2 != 0 {
		panic("required key-value pair of entries")
	}
	for i := 0; i < len(entries); i += 2 {
		(*tags)[entries[i]] = entries[i+1]
	}
	return tags
}

func TrackFinishedWorkflow(ctx workflow.Context, workflowStatus string) {
	start := workflow.GetInfo(ctx).WorkflowStartTime
	stop := workflow.Now(ctx)
	duration := stop.Sub(start)

	workflow.GetMetricsHandler(ctx).WithTags(map[string]string{"wfStatus": workflowStatus}).Timer("workflow_finished").Record(duration)
}

func TrackFinishedTask[T any](ctx workflow.Context, taskName string, activityRunner func() TaskResult[T]) (T, error) {
	start := workflow.Now(ctx)

	result := activityRunner()

	duration := workflow.Now(ctx).Sub(start)
	tags := map[string]string{
		"wfTaskStatus": result.Status,
		"attempts":     strconv.Itoa(result.Attempts),
		"task":         taskName,
	}
	workflow.GetMetricsHandler(ctx).WithTags(tags).Timer("workflow_task_finished").Record(duration)
	return result.Result, result.Err
}
