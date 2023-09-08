package temporal

import (
	"context"
	"github.com/armory-io/go-commons/server"
	"github.com/samber/lo"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
	"sync"
)

const loggingPropagationKey = "armory-logging"

type (
	loggerContextKey struct{}

	loggerContextPropagator struct{}

	LoggerField struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
)

func ExtractLoggerMetadata(header *common.Header) (map[string]string, error) {
	loggingMetadata, ok := header.Fields[loggingPropagationKey]
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
	fields = append(fields, extractFields(ctx)...)
	return context.WithValue(ctx, loggerContextKey{}, setFields(ctx, fields...))
}

func WithWorkflowFields(ctx workflow.Context, fields ...LoggerField) workflow.Context {
	fields = append(fields, extractFields(ctx)...)
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
	writer.Set(loggingPropagationKey, payload)
	return nil
}

func (p *loggerContextPropagator) Extract(ctx context.Context, reader workflow.HeaderReader) (context.Context, error) {
	if raw, ok := reader.Get(loggingPropagationKey); ok {
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
	writer.Set(loggingPropagationKey, payload)
	return nil
}

func (p *loggerContextPropagator) ExtractToWorkflow(ctx workflow.Context, reader workflow.HeaderReader) (workflow.Context, error) {
	if raw, ok := reader.Get(loggingPropagationKey); ok {
		var fields []LoggerField
		if err := converter.GetDefaultDataConverter().FromPayload(raw, &fields); err != nil {
			return ctx, nil
		}
		ctx = workflow.WithValue(ctx, loggerContextKey{}, setFields(ctx, fields...))
	}
	return ctx, nil
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

func extractFields(ctx valuer) []LoggerField {
	details, err := server.ExtractRequestDetailsFromContext(ctx)
	if err != nil {
		return []LoggerField{}
	}
	loggingMetadata := details.LoggingMetadata.Metadata
	return lo.MapToSlice(loggingMetadata, func(k string, v string) LoggerField {
		return LoggerField{
			Key:   k,
			Value: v,
		}
	})
}
