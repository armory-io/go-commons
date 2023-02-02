/*
 * Copyright 2022 Armory, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package metrics

import (
	"context"
	"errors"
	"fmt"
	"github.com/armory-io/go-commons/metadata"
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

// NewSvc creates an instance of the metrics service but does not start a server for metrics scraping.
// Serving the open metrics endpoint is handled by a management endpoint, see the management package.
func NewSvc(lc fx.Lifecycle, app metadata.ApplicationMetadata) MetricsSvc {
	registerer := prometheus.DefaultRegisterer
	reporter := tallyprom.NewReporter(tallyprom.Options{Registerer: registerer})
	scopeOpts := tally.ScopeOptions{
		CachedReporter:  reporter,
		Separator:       tallyprom.DefaultSeparator,
		SanitizeOptions: &sanitizeOptions,
		Tags: map[string]string{
			"service.name": app.Name, // <- service.name is required to link custom metrics with otel trace and log data
			"appName":      app.Name, // <- this duplicates service.name, but I don't want to break existing dashboards and alerts
			"version":      app.Version,
			"hostname":     app.Hostname,
			"environment":  app.Environment,
			"replicaset":   app.Replicaset,
			"deploymentId": app.DeploymentId,
		},
	}
	scope, closer := tally.NewRootScope(scopeOpts, time.Second)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return closer.Close()
		},
	})

	s := &Metrics{
		rootScope: scope,
	}

	return s
}

// New creates a metrics service that by defaults serves metrics on :3001/metrics, but is separate from the management endpoints
// Deprecated: this will be deleted once all apps are on the server module, where metrics will be served on the management port (defaults to the server port unless you change it)
func New(lc fx.Lifecycle, log *zap.SugaredLogger, conf Configuration, app metadata.ApplicationMetadata) MetricsSvc {
	path := conf.Path
	port := conf.Port

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
		Tags: map[string]string{
			"appName":     app.Name,
			"version":     app.Version,
			"hostname":    app.Hostname,
			"environment": app.Environment,
			"replicaset":  app.Replicaset,
		},
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
					if !errors.Is(err, http.ErrServerClosed) {
						log.Fatalf("Failed to start metrics server: %s", err)
					}
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

func (s *Metrics) Tagged(tags map[string]string) tally.Scope {
	return s.rootScope.Tagged(tags)
}

func (s *Metrics) SubScope(name string) tally.Scope {
	return s.rootScope.SubScope(name)
}

func (s *Metrics) Capabilities() tally.Capabilities {
	return s.rootScope.Capabilities()
}
