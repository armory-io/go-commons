package temporal

import (
	"context"
	"github.com/armory-io/go-commons/iam"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"testing"
	"time"
)

type PrincipalPropagatorTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestIAMTestSuite(t *testing.T) {
	suite.Run(t, new(PrincipalPropagatorTestSuite))
}

func (p *PrincipalPropagatorTestSuite) TestWorkflowPrincipalPropagation() {
	env := p.NewTestWorkflowEnvironment()
	env.SetContextPropagators([]workflow.ContextPropagator{NewPrincipalContextPropagator()})
	env.RegisterWorkflow(principalWorkflow)

	p.NoError(InitializeTestWorkflowEnvironmentContextWithPrincipal(env, iam.ArmoryCloudPrincipal{Name: "banana"}))

	env.ExecuteWorkflow(principalWorkflow)

	p.NoError(env.GetWorkflowError())
	var principal iam.ArmoryCloudPrincipal
	p.NoError(env.GetWorkflowResult(&principal))
	p.Equal("banana", principal.Name)
}

func (p *PrincipalPropagatorTestSuite) TestActivityPrincipalPropagation() {
	env := p.NewTestWorkflowEnvironment()
	env.SetContextPropagators([]workflow.ContextPropagator{NewPrincipalContextPropagator()})
	env.RegisterWorkflow(parentWorkflow)
	env.RegisterActivity(principalActivity)

	p.NoError(InitializeTestWorkflowEnvironmentContextWithPrincipal(env, iam.ArmoryCloudPrincipal{Name: "banana"}))

	env.ExecuteWorkflow(parentWorkflow)

	p.NoError(env.GetWorkflowError())
	var principal iam.ArmoryCloudPrincipal
	p.NoError(env.GetWorkflowResult(&principal))
	p.Equal("banana", principal.Name)
}

func parentWorkflow(ctx workflow.Context) (*iam.ArmoryCloudPrincipal, error) {
	var principal iam.ArmoryCloudPrincipal
	if err := workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Second}),
		principalActivity,
	).Get(ctx, &principal); err != nil {
		return nil, err
	}

	return &principal, nil
}

func principalActivity(ctx context.Context) (*iam.ArmoryCloudPrincipal, error) {
	return iam.ExtractPrincipalFromContext(ctx)
}

func principalWorkflow(ctx workflow.Context) (*iam.ArmoryCloudPrincipal, error) {
	return iam.ExtractPrincipalFromContext(ctx)
}
