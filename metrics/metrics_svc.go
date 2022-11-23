package metrics

import "github.com/uber-go/tally/v4"

//go:generate mockgen -package metrics -destination=mock_metrics.go -source=./metrics_svc.go MetricsSvc

type MetricsSvc interface {
	Counter(name string) tally.Counter
	CounterWithTags(name string, tags map[string]string) tally.Counter
	Gauge(name string) tally.Gauge
	GaugeWithTags(name string, tags map[string]string) tally.Gauge
	Timer(name string) tally.Timer
	TimerWithTags(name string, tags map[string]string) tally.Timer
	Histogram(name string, buckets tally.Buckets) tally.Histogram
	HistogramWithTags(name string, buckets tally.Buckets, tags map[string]string) tally.Histogram
}
