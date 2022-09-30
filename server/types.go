package server

import (
	"context"
	"github.com/armory-io/go-commons/iam"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type (
	controllerConfig struct {
		prefix         string
		authZValidator AuthZValidator
	}

	Handler interface {
		Register(g gin.IRoutes, log *zap.SugaredLogger, v *validator.Validate, config *controllerConfig)
		Config() HandlerConfig
	}

	HandlerConfig struct {
		Path           string
		Method         string
		AuthZValidator AuthZValidator
		AuthOptOut     bool
		Consumes       string
		Produces       string
		StatusCode     int
	}

	AuthZValidator func(p *iam.ArmoryCloudPrincipal) (string, bool)
	RequestFilter  func(ctx context.Context, request any, response any) Error

	handler[T, U any] struct {
		config     HandlerConfig
		handleFunc func(ctx context.Context, request T) (*Response[U], Error)
	}

	// controller baseController the base controller interface, all controllers must impl this via providing an instance of Controller or ManagementController
	controller interface {
		Handlers() []Handler
	}

	// ControllerPrefix a controller can implement this interface to have all of its handler func paths prefixed with a common path partial
	ControllerPrefix interface {
		Prefix() string
	}

	// ControllerAuthZValidator a controller can implement this interface to apply a common AuthZ validator to all exported handlers
	ControllerAuthZValidator interface {
		AuthZValidator(p *iam.ArmoryCloudPrincipal) (string, bool)
	}

	Controller struct {
		fx.Out
		Controller controller `group:"server"`
	}

	Controllers struct {
		fx.In
		Controllers []controller `group:"server"`
	}

	ManagementController struct {
		fx.Out
		Controller controller `group:"management"`
	}

	ManagementControllers struct {
		fx.In
		Controllers []controller `group:"management"`
	}

	Void struct{}

	Request[T any] struct {
		Headers map[string][]string
		Body    T
	}

	Response[T any] struct {
		StatusCode int
		Headers    map[string][]string
		Body       T
	}
)

func SimpleResponse[T any](body T) *Response[T] {
	return &Response[T]{
		Body: body,
	}
}
