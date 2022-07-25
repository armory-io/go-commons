package metrics

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/uber-go/tally/v4"
	tallyprom "github.com/uber-go/tally/v4/prometheus"
	"go.uber.org/fx"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"net/http"
	"time"
)

var (
	safeCharacters = []rune{'_', '-', '/', ':'}

	sanitizeOptions = tally.SanitizeOptions{
		NameCharacters: tally.ValidCharacters{
			Ranges:     tally.AlphanumericRange,
			Characters: safeCharacters,
		},
		KeyCharacters: tally.ValidCharacters{
			Ranges:     tally.AlphanumericRange,
			Characters: safeCharacters,
		},
		ValueCharacters: tally.ValidCharacters{
			Ranges:     tally.AlphanumericRange,
			Characters: safeCharacters,
		},
		ReplacementCharacter: tally.DefaultReplacementCharacter,
	}
)

type Metrics struct {
	log       *logrus.Logger
	rootScope tally.Scope
}

func New(lc fx.Lifecycle, log *zap.SugaredLogger, settings Settings) *Metrics {
	path := settings.Path
	port := settings.Port

	if path == "" {
		path = "/metrics"
	}

	if port == "" {
		port = "3001"
	}

	registerer := prometheus.DefaultRegisterer

	reporter := tallyprom.NewReporter(tallyprom.Options{Registerer: registerer})
	scopeOpts := tally.ScopeOptions{
		CachedReporter:  reporter,
		Separator:       tallyprom.DefaultSeparator,
		SanitizeOptions: &sanitizeOptions,
		Tags:            getBaseTags(settings),
	}

	scope, closer := tally.NewRootScope(scopeOpts, time.Second)

	s := &Metrics{
		rootScope: scope,
	}

	mux := http.NewServeMux()
	mux.Handle(path, promhttp.Handler())
	addr := fmt.Sprintf(":%s", port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				if err := server.ListenAndServe(); err != nil {
					log.Fatalf("Failed to start metrics server: %s", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return multierr.Combine(
				server.Shutdown(ctx),
				closer.Close(),
			)
		},
	})

	return s
}

// GetRootScope gets the root scope with the configured base tags
func (s *Metrics) GetRootScope() tally.Scope {
	return s.rootScope
}

func (s *Metrics) Counter(name string) tally.Counter {
	return s.rootScope.Counter(name)
}

func (s *Metrics) CounterWithTags(name string, tags map[string]string) tally.Counter {
	if len(tags) < 1 {
		return s.Counter(name)
	}
	return s.rootScope.Tagged(tags).Counter(name)
}

func (s *Metrics) Gauge(name string) tally.Gauge {
	return s.rootScope.Gauge(name)
}

func (s *Metrics) GaugeWithTags(name string, tags map[string]string) tally.Gauge {
	if len(tags) < 1 {
		return s.Gauge(name)
	}
	return s.rootScope.Tagged(tags).Gauge(name)
}

func (s *Metrics) Timer(name string) tally.Timer {
	return s.rootScope.Timer(name)
}

func (s *Metrics) TimerWithTags(name string, tags map[string]string) tally.Timer {
	if len(tags) < 1 {
		return s.Timer(name)
	}
	return s.rootScope.Tagged(tags).Timer(name)
}

func (s *Metrics) Histogram(name string, buckets tally.Buckets) tally.Histogram {
	return s.rootScope.Histogram(name, buckets)
}

func (s *Metrics) HistogramWithTags(name string, buckets tally.Buckets, tags map[string]string) tally.Histogram {
	if len(tags) < 1 {
		return s.Histogram(name, buckets)
	}
	return s.rootScope.Tagged(tags).Histogram(name, buckets)
}
