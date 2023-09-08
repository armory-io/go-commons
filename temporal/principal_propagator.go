package temporal

import (
	"context"
	"github.com/armory-io/go-commons/iam"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

const (
	principalPropagationKey = "armory-principal"
)

type (
	principalContextPropagator struct{}
)

func NewPrincipalContextPropagator() workflow.ContextPropagator {
	return &principalContextPropagator{}
}

// InitializeTestWorkflowEnvironmentContextWithPrincipal should be only be used in tests.
func InitializeTestWorkflowEnvironmentContextWithPrincipal(env *testsuite.TestWorkflowEnvironment, principal iam.ArmoryCloudPrincipal) error {
	// Ordinarily this would come in through the context.Context passed to workflow start function,
	// but there's no way to pass an initial context to a Temporal test workflow.
	payload, err := converter.GetDefaultDataConverter().ToPayload(principal)
	if err != nil {
		return err
	}

	header := &commonpb.Header{
		Fields: map[string]*commonpb.Payload{
			principalPropagationKey: payload,
		},
	}
	env.SetHeader(header)
	return nil
}

func (p *principalContextPropagator) Inject(ctx context.Context, writer workflow.HeaderWriter) error {
	principal, err := iam.ExtractPrincipalFromContext(ctx)
	if err != nil {
		return nil
	}
	payload, err := converter.GetDefaultDataConverter().ToPayload(principal)
	if err != nil {
		return err
	}
	writer.Set(principalPropagationKey, payload)
	return nil
}

func (p *principalContextPropagator) Extract(ctx context.Context, reader workflow.HeaderReader) (context.Context, error) {
	if raw, ok := reader.Get(principalPropagationKey); ok {
		var principal iam.ArmoryCloudPrincipal
		if err := converter.GetDefaultDataConverter().FromPayload(raw, &principal); err != nil {
			return ctx, nil
		}
		ctx = iam.WithPrincipal(ctx, principal)
	}
	return ctx, nil
}

func (p *principalContextPropagator) InjectFromWorkflow(ctx workflow.Context, writer workflow.HeaderWriter) error {
	principal, err := iam.ExtractPrincipalFromContext(ctx)
	if err != nil {
		return nil
	}

	payload, err := converter.GetDefaultDataConverter().ToPayload(principal)
	if err != nil {
		return err
	}
	writer.Set(principalPropagationKey, payload)
	return nil
}

func (p *principalContextPropagator) ExtractToWorkflow(ctx workflow.Context, reader workflow.HeaderReader) (workflow.Context, error) {
	if raw, ok := reader.Get(principalPropagationKey); ok {
		var principal iam.ArmoryCloudPrincipal
		if err := converter.GetDefaultDataConverter().FromPayload(raw, &principal); err != nil {
			return ctx, nil
		}
		ctx = iam.WithPrincipalWorkflow(ctx, principal)
	}
	return ctx, nil
}
