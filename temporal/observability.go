package temporal

import (
	"context"
	"github.com/samber/lo"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/workflow"
	"strconv"
)

type (
	workflowObservabilityParametersKey        struct{}
	workflowObservabilityParametersPropagator struct{}

	ActivityResult[T any] struct {
		Result   T
		Err      error
		Attempts int
		Status   string
	}
)

const (
	workflowFinishedMetric         = "workflow_finished"
	workflowActivityFinishedMetric = "workflow_activity_finished"
	observabilityKey               = "armory-observability"
	attemptsTag                    = "attempts"
	workflowStatusTag              = "wfStatus"
	workflowActivityStatusTag      = "wfActivityStatus"
	activityNameTag                = "activityName"
)

func NewActivitySuccessResult[T any](result T, attempts int, status string) ActivityResult[T] {
	return ActivityResult[T]{
		Result:   result,
		Attempts: attempts,
		Status:   status,
	}
}

func NewActivityErrorResult[T any](err error, attempts int, status string) ActivityResult[T] {
	return ActivityResult[T]{
		Err:      err,
		Attempts: attempts,
		Status:   status,
	}
}

func WithObservabilityParameters(ctx context.Context, entries ...string) context.Context {
	container := getOrCreateTags(ctx.Value)
	return context.WithValue(ctx, workflowObservabilityParametersKey{}, makeTraceabilityTags(container, entries))
}

func WithWorkflowObservabilityParameters(ctx workflow.Context, entries ...string) workflow.Context {
	container := getOrCreateTags(ctx.Value)
	return workflow.WithValue(ctx, workflowObservabilityParametersKey{}, makeTraceabilityTags(container, entries))
}

func TrackFinishedWorkflow(ctx workflow.Context, workflowStatus string) {
	start := workflow.GetInfo(ctx).WorkflowStartTime
	stop := workflow.Now(ctx)
	duration := stop.Sub(start)

	workflow.GetMetricsHandler(ctx).WithTags(map[string]string{workflowStatusTag: workflowStatus}).Timer(workflowFinishedMetric).Record(duration)
}

func TrackFinishedActivity[T any](ctx workflow.Context, activityName string, activityRunner func() ActivityResult[T]) (T, error) {
	start := workflow.Now(ctx)

	result := activityRunner()

	duration := workflow.Now(ctx).Sub(start)
	tags := map[string]string{
		workflowActivityStatusTag: result.Status,
		attemptsTag:               strconv.Itoa(result.Attempts),
		activityNameTag:           activityName,
	}
	workflow.GetMetricsHandler(ctx).WithTags(tags).Timer(workflowActivityFinishedMetric).Record(duration)
	return result.Result, result.Err
}

func (w workflowObservabilityParametersPropagator) ExtractToWorkflow(ctx workflow.Context, reader workflow.HeaderReader) (workflow.Context, error) {
	if raw, ok := reader.Get(observabilityKey); ok {
		var kvps map[string]string
		if err := converter.GetDefaultDataConverter().FromPayload(raw, &kvps); err != nil {
			return ctx, nil
		}
		ctx = workflow.WithValue(ctx, workflowObservabilityParametersKey{}, kvps)
	}
	return ctx, nil
}

func newWorkflowObservabilityParametersPropagator() workflow.ContextPropagator {
	return &workflowObservabilityParametersPropagator{}
}

func (w workflowObservabilityParametersPropagator) Inject(ctx context.Context, writer workflow.HeaderWriter) error {
	kvp, ok := ctx.Value(workflowObservabilityParametersKey{}).(map[string]string)
	if ok && nil != kvp {
		payload, err := converter.GetDefaultDataConverter().ToPayload(kvp)
		if err != nil {
			return err
		}
		writer.Set(observabilityKey, payload)
	}
	return nil
}

func (w workflowObservabilityParametersPropagator) Extract(ctx context.Context, reader workflow.HeaderReader) (context.Context, error) {
	if raw, ok := reader.Get(observabilityKey); ok {
		var kvp map[string]string
		if err := converter.GetDefaultDataConverter().FromPayload(raw, &kvp); err != nil {
			return ctx, nil
		}
		ctx = context.WithValue(ctx, workflowObservabilityParametersKey{}, kvp)
	}
	return ctx, nil
}

func (w workflowObservabilityParametersPropagator) InjectFromWorkflow(ctx workflow.Context, writer workflow.HeaderWriter) error {
	kvp, ok := ctx.Value(workflowObservabilityParametersKey{}).(map[string]string)
	if ok {
		payload, err := converter.GetDefaultDataConverter().ToPayload(kvp)
		if err != nil {
			return err
		}
		writer.Set(observabilityKey, payload)
	}
	return nil
}

func getOrCreateTags(contextGetter func(key any) any) map[string]string {
	existing, ok := contextGetter(workflowObservabilityParametersKey{}).(map[string]string)
	return lo.IfF(ok && existing != nil, func() map[string]string {
		copy := map[string]string{}
		for k, v := range existing {
			copy[k] = v
		}
		return copy
	}).ElseF(func() map[string]string {
		return map[string]string{}
	})
}

func makeTraceabilityTags(tags map[string]string, entries []string) map[string]string {
	for i := 1; i < len(entries); i += 2 {
		tags[entries[i-1]] = entries[i]
	}
	return tags
}

func getTagsFromWorkflowContext(ctx workflow.Context) map[string]string {
	if tags, ok := ctx.Value(workflowObservabilityParametersKey{}).(map[string]string); ok && tags != nil {
		return tags
	}
	return map[string]string{}
}

func getTagsFromContext(ctx context.Context) map[string]string {
	if tags, ok := ctx.Value(workflowObservabilityParametersKey{}).(map[string]string); ok && tags != nil {
		return tags
	}
	return map[string]string{}
}

func withTags(handler client.MetricsHandler, tags map[string]string) client.MetricsHandler {
	return handler.WithTags(tags)
}
