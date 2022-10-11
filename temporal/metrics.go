package temporal

import (
	"github.com/armory-io/go-commons/metrics"
	"go.temporal.io/sdk/client"
	temporaltally "go.temporal.io/sdk/contrib/tally"
)

func newMetricsHandler(metricsService *metrics.Metrics) client.MetricsHandler {
	return temporaltally.NewMetricsHandler(metricsService.GetRootScope().SubScope("armory_temporal_sdk"))
}
