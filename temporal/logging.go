package temporal

import (
	"context"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
	"sync"
)

const propagationKey = "armory-logging"

type (
	loggerContextKey struct{}

	loggerContextPropagator struct{}

	LoggerField struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
)

func ExtractLoggerMetadata(header *common.Header) (map[string]string, error) {
	loggingMetadata, ok := header.Fields[propagationKey]
	if !ok {
		return make(map[string]string), nil
	}

	var fields []LoggerField
	if err := converter.GetDefaultDataConverter().FromPayload(loggingMetadata, &fields); err != nil {
		return nil, err
	}

	out := make(map[string]string)
	for _, field := range fields {
		out[field.Key] = field.Value
	}
	return out, nil
}

func WithFields(ctx context.Context, fields ...LoggerField) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, setFields(ctx, fields...))
}

func WithWorkflowFields(ctx workflow.Context, fields ...LoggerField) workflow.Context {
	return workflow.WithValue(ctx, loggerContextKey{}, setFields(ctx, fields...))
}

func NewLoggerContextPropagator() workflow.ContextPropagator {
	return &loggerContextPropagator{}
}

func (p *loggerContextPropagator) Inject(ctx context.Context, writer workflow.HeaderWriter) error {
	fields := getFields(ctx)

	payload, err := converter.GetDefaultDataConverter().ToPayload(fields)
	if err != nil {
		return err
	}
	writer.Set(propagationKey, payload)
	return nil
}

func (p *loggerContextPropagator) Extract(ctx context.Context, reader workflow.HeaderReader) (context.Context, error) {
	if raw, ok := reader.Get(propagationKey); ok {
		var fields []LoggerField
		if err := converter.GetDefaultDataConverter().FromPayload(raw, &fields); err != nil {
			return ctx, nil
		}
		ctx = context.WithValue(ctx, loggerContextKey{}, setFields(ctx, fields...))
	}
	return ctx, nil
}

func (p *loggerContextPropagator) InjectFromWorkflow(ctx workflow.Context, writer workflow.HeaderWriter) error {
	fields := getFields(ctx)

	payload, err := converter.GetDefaultDataConverter().ToPayload(fields)
	if err != nil {
		return err
	}
	writer.Set(propagationKey, payload)
	return nil
}

func (p *loggerContextPropagator) ExtractToWorkflow(ctx workflow.Context, reader workflow.HeaderReader) (workflow.Context, error) {
	if raw, ok := reader.Get(propagationKey); ok {
		var fields []LoggerField
		if err := converter.GetDefaultDataConverter().FromPayload(raw, &fields); err != nil {
			return ctx, nil
		}
		ctx = workflow.WithValue(ctx, loggerContextKey{}, setFields(ctx, fields...))
	}
	return ctx, nil
}

type loggerInterceptor struct {
	interceptor.WorkerInterceptorBase
}

func NewLoggerInterceptor() interceptor.WorkerInterceptor {
	return &loggerInterceptor{}
}

func (w *loggerInterceptor) InterceptActivity(
	ctx context.Context,
	next interceptor.ActivityInboundInterceptor,
) interceptor.ActivityInboundInterceptor {
	i := &activityInboundLoggerInterceptor{root: w}
	i.Next = next
	return i
}

type activityInboundLoggerInterceptor struct {
	interceptor.ActivityInboundInterceptorBase
	root *loggerInterceptor
}

func (a *activityInboundLoggerInterceptor) Init(outbound interceptor.ActivityOutboundInterceptor) error {
	i := &activityOutboundLoggerInterceptor{root: a.root}
	i.Next = outbound
	return a.Next.Init(i)
}

type activityOutboundLoggerInterceptor struct {
	interceptor.ActivityOutboundInterceptorBase
	root *loggerInterceptor
}

func (a *activityOutboundLoggerInterceptor) GetLogger(ctx context.Context) log.Logger {
	logger := a.Next.GetLogger(ctx)
	return withFields(logger, getFields(ctx))
}

func (w *loggerInterceptor) InterceptWorkflow(
	ctx workflow.Context,
	next interceptor.WorkflowInboundInterceptor,
) interceptor.WorkflowInboundInterceptor {
	i := &workflowInboundLoggerInterceptor{root: w}
	i.Next = next
	return i
}

type workflowInboundLoggerInterceptor struct {
	interceptor.WorkflowInboundInterceptorBase
	root *loggerInterceptor
}

func (w *workflowInboundLoggerInterceptor) Init(outbound interceptor.WorkflowOutboundInterceptor) error {
	i := &workflowOutboundLoggerInterceptor{root: w.root}
	i.Next = outbound
	return w.Next.Init(i)
}

type workflowOutboundLoggerInterceptor struct {
	interceptor.WorkflowOutboundInterceptorBase
	root *loggerInterceptor
}

func (w *workflowOutboundLoggerInterceptor) GetLogger(ctx workflow.Context) log.Logger {
	logger := w.Next.GetLogger(ctx)
	return withFields(logger, getFields(ctx))
}

type valuer interface {
	Value(any) any
}

func getFields(ctx valuer) []LoggerField {
	m, ok := ctx.Value(loggerContextKey{}).(*sync.Map)
	if !ok {
		return nil
	}
	var fields []LoggerField
	m.Range(func(key, value any) bool {
		fields = append(fields, LoggerField{
			Key:   key.(string),
			Value: value.(string),
		})
		return true
	})
	return fields
}

func withFields(logger log.Logger, fields []LoggerField) log.Logger {
	var raw []interface{}
	for _, kv := range fields {
		raw = append(raw, kv.Key, kv.Value)
	}
	return log.With(logger, raw...)
}

func setFields(ctx valuer, fields ...LoggerField) *sync.Map {
	m, ok := ctx.Value(loggerContextKey{}).(*sync.Map)
	if !ok {
		m = &sync.Map{}
	}

	for _, field := range fields {
		m.Store(field.Key, field.Value)
	}

	return m
}
