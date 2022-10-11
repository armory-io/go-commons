package management

import (
	"context"
	"github.com/armory-io/go-commons/server"
	"github.com/armory-io/go-commons/server/serr"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net/http"
)

type HealthController struct {
	log              *zap.SugaredLogger
	healthIndicators []indicator
}

type HealthCheckResponse struct {
	Status string         `json:"status"`
	Info   map[string]any `json:"info,omitempty"`
}

func NewHealthCheckController(log *zap.SugaredLogger, i Indicators) server.ManagementController {
	return server.ManagementController{
		Controller: &HealthController{
			log:              log,
			healthIndicators: i.HealthIndicators,
		},
	}
}

// indicator Strategy interface used to contribute Health to the results returned from the HealthEndpoint.
type indicator interface {
	Health() *Health
}

// Health Carries information about the health of a component or subsystem.
type Health struct {
	Name  string
	Ready bool
	Alive bool
	Msg   string
}

type Indicators struct {
	fx.In
	HealthIndicators []indicator `group:"health-check"`
}

type Indicator struct {
	fx.Out
	HealthIndicator indicator `group:"health-check"`
}

func (c *HealthController) Handlers() []server.Handler {
	return []server.Handler{
		server.NewHandler(c.readinessCheckHandler, server.HandlerConfig{
			Path:       "/health/readiness",
			Method:     http.MethodGet,
			AuthOptOut: true,
		}),
		server.NewHandler(c.livenessCheckHandler, server.HandlerConfig{
			Path:       "/health/liveness",
			Method:     http.MethodGet,
			AuthOptOut: true,
		}),
	}
}

func (c *HealthController) readinessCheckHandler(_ context.Context, _ server.Void) (*server.Response[HealthCheckResponse], serr.Error) {
	statusCode := http.StatusServiceUnavailable
	status := "unavailable"
	isReady := true
	var info map[string]any
	for _, indicator := range c.healthIndicators {
		health := indicator.Health()
		if !health.Ready {
			isReady = false
		}
		info[health.Name] = map[string]any{
			"ready": health.Ready,
			"msg":   health.Msg,
		}
	}

	if isReady {
		statusCode = http.StatusOK
		status = "ok"
	}

	return &server.Response[HealthCheckResponse]{
		Body: HealthCheckResponse{
			Status: status,
			Info:   info,
		},
		StatusCode: statusCode,
	}, nil
}

func (c *HealthController) livenessCheckHandler(_ context.Context, _ server.Void) (*server.Response[HealthCheckResponse], serr.Error) {
	statusCode := http.StatusServiceUnavailable
	status := "unavailable"
	isAlive := true
	var info map[string]any
	for _, indicator := range c.healthIndicators {
		health := indicator.Health()
		if !health.Alive {
			isAlive = false
		}
		info[health.Name] = map[string]any{
			"alive": health.Alive,
			"msg":   health.Msg,
		}
	}

	if isAlive {
		statusCode = http.StatusOK
		status = "ok"
	}

	return &server.Response[HealthCheckResponse]{
		Body: HealthCheckResponse{
			Status: status,
			Info:   info,
		},
		StatusCode: statusCode,
	}, nil
}
