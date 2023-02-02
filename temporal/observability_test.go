package temporal

import (
	"context"
	"github.com/armory-io/go-commons/metrics"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally/v4"
	"go.temporal.io/api/common/v1"
	temporaltally "go.temporal.io/sdk/contrib/tally"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"testing"
	"time"
)

type UnitTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestUnitTestSuite(t *testing.T) {
	suite.Run(t, new(UnitTestSuite))
}

type observabilityTestContext struct {
	name string
	run  func(t *testing.T, testCtx *observabilityTestContext)
}

type testHeaderReaderWriter struct {
	key     string
	payload *common.Payload
}

type testWorkflowContext struct {
	workflow.Context
}

type testTimer struct {
}

func (*testWorkflowContext) Value(key interface{}) interface{} {
	return nil
}

func (w *testHeaderReaderWriter) Set(key string, payload *common.Payload) {
	w.payload = payload
	w.key = key
}

func (w *testHeaderReaderWriter) Get(key string) (*common.Payload, bool) {
	if key == w.key {
		return w.payload, true
	}
	return nil, false
}

func (w *testHeaderReaderWriter) ForEachKey(handler func(key string, payload *common.Payload) error) error {
	if w.key != "" {
		return handler(w.key, w.payload)
	}
	return nil
}

func (testTimer) Record(_ time.Duration) {
}

func (testTimer) Start() tally.Stopwatch {
	return tally.Stopwatch{}
}

func TestExtractObservabilityData(t *testing.T) {

	propagator := newWorkflowObservabilityParametersPropagator()
	headerReaderWriter := &testHeaderReaderWriter{}

	cases := []observabilityTestContext{
		{
			name: "inject from plain context to header",
			run: func(t *testing.T, testCtx *observabilityTestContext) {
				ctx := WithObservabilityParameters(context.TODO(), "first", "entry", "hello", "world")
				err := propagator.Inject(ctx, headerReaderWriter)

				assert.NoError(t, err)
				assert.Equal(t, headerReaderWriter.key, observabilityKey)
			},
		},
		{
			name: "extract from header to plain context",
			run: func(t *testing.T, testCtx *observabilityTestContext) {
				ctx, err := propagator.Extract(context.TODO(), headerReaderWriter)
				assert.NoError(t, err)
				tags := getTagsFromContext(ctx)
				assert.Equal(t, tags["first"], "entry")
				assert.Equal(t, tags["hello"], "world")
			},
		},
		{
			name: "inject from workflow to header",
			run: func(t *testing.T, testCtx *observabilityTestContext) {
				ctx := WithWorkflowObservabilityParameters(&testWorkflowContext{}, "second", "entry", "hello", "workflow")
				err := propagator.InjectFromWorkflow(ctx, headerReaderWriter)
				assert.NoError(t, err)
				assert.Equal(t, headerReaderWriter.key, observabilityKey)
			},
		},
		{
			name: "extract from header to workflow context",
			run: func(t *testing.T, testCtx *observabilityTestContext) {
				ctx, err := propagator.ExtractToWorkflow(&testWorkflowContext{}, headerReaderWriter)
				assert.NoError(t, err)
				tags := getTagsFromWorkflowContext(ctx)
				assert.Equal(t, tags["second"], "entry")
				assert.Equal(t, tags["hello"], "workflow")
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.run(t, &c)
		})
	}

}

func (s *UnitTestSuite) TestObservabilityWorksOnSampleWorkflowAndActivity() {
	m := metrics.NewMockMetricsSvc(gomock.NewController(s.T()))
	m.EXPECT().Tagged(gomock.Any()).Return(m).AnyTimes()
	testTimer := testTimer{}
	m.EXPECT().Timer(workflowFinishedMetric).Return(&testTimer).Times(1)
	m.EXPECT().Timer(workflowActivityFinishedMetric).Return(&testTimer).Times(2)
	s.SetMetricsHandler(temporaltally.NewMetricsHandler(m))

	opts := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
	}
	testWorkflow := func(ctx workflow.Context) (string, error) {
		ctx = WithWorkflowObservabilityParameters(ctx, "arg1", "1", "arg2", "2")

		result1, _ := TrackFinishedActivity[string](ctx, "some-activity-01", func() ActivityResult[string] {
			result := ""
			_ = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, opts), SomeActivity, "arg1").Get(ctx, &result)
			return NewActivitySuccessResult(result, 1, "ok")
		})

		result2, _ := TrackFinishedActivity[string](ctx, "some-activity-02", func() ActivityResult[string] {
			result := ""
			_ = workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, opts), SomeActivity, "arg2").Get(ctx, &result)
			return NewActivitySuccessResult(result, 1, "ok")
		})

		TrackFinishedWorkflow(ctx, "ok")

		return result1 + "." + result2, nil
	}

	env := s.NewTestWorkflowEnvironment()
	env.SetContextPropagators([]workflow.ContextPropagator{newWorkflowObservabilityParametersPropagator()})
	env.SetWorkerOptions(worker.Options{
		Interceptors: []interceptor.WorkerInterceptor{newWorkflowContextInterceptor()},
	})

	env.RegisterWorkflow(testWorkflow)
	env.RegisterActivity(SomeActivity)

	env.ExecuteWorkflow(testWorkflow)

	s.True(env.IsWorkflowCompleted())
	err := env.GetWorkflowError()
	s.NoError(err)
	result := ""
	_ = env.GetWorkflowResult(&result)
	s.Equal("test=1.test=2", result)
}

func SomeActivity(ctx context.Context, tag string) (string, error) {
	tags := getTagsFromContext(ctx)
	result := "test=" + tags[tag]
	time.Sleep(time.Millisecond * 100)
	return result, nil
}
