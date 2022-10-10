package server

import (
	"context"
	"github.com/armory-io/go-commons/iam"
	"github.com/armory-io/go-commons/server/serr"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

type (
	// Handler The handler interface
	// Instances of this interface should only ever be created by NewHandler, which happens automatically during server initialization
	// The expected way that handlers are created is by creating a provider that provides an instance of Controller
	Handler interface {
		GetGinHandlerFn(log *zap.SugaredLogger, v *validator.Validate, handler *handlerDTO) gin.HandlerFunc
		Config() HandlerConfig
	}

	// HandlerConfig config that configures a handler AKA an endpoint
	HandlerConfig struct {
		// Path The path or sub-path if a root path is set on the controller that the handler will be served on
		Path string
		// Method The HTTP method that the handler will be served on
		Method string
		// Consumes The content-type that the handler consumes, defaults to application/json
		Consumes string
		// Produces The content-type that the handler produces/offers, defaults to application/json
		Produces string
		// Default denotes that the handler should be used when the request doesn't specify a preferred Media/MIME type via the Accept header
		// Please note that one and only one handler for a given path/method combo can be marked as default, else a runtime error will be produced.
		Default bool
		// StatusCode The default status code to return when the request is successful, can be overridden by the handler by setting Response.StatusCode in the handler
		StatusCode int
		// AuthOptOut Set this to true if the handler should skip AuthZ and AuthN, this will cause the principal to be nil in the request context
		AuthOptOut bool
		// AuthZValidator see AuthZValidatorFn
		AuthZValidator AuthZValidatorFn
	}

	// AuthZValidatorFn a function that takes the authenticated principal and returns whether the principal is authorized.
	// return true if the user is authorized
	// return false if the user is NOT authorized and a string indicated the reason.
	AuthZValidatorFn func(p *iam.ArmoryCloudPrincipal) (string, bool)

	handler[T, U any] struct {
		config     HandlerConfig
		handleFunc func(ctx context.Context, request T) (*Response[U], serr.Error)
	}
)

func (r *handler[REQUEST, RESPONSE]) Config() HandlerConfig {
	return r.config
}

func (r *handler[REQUEST, RESPONSE]) GetGinHandlerFn(log *zap.SugaredLogger, requestValidator *validator.Validate, config *handlerDTO) gin.HandlerFunc {
	return createGinFunctionFromHandlerFn(r.handleFunc, config, requestValidator, log)
}

// NewHandler creates a Handler from a handler function and server.HandlerConfig
func NewHandler[REQUEST, RESPONSE any](f func(ctx context.Context, request REQUEST) (*Response[RESPONSE], serr.Error), config HandlerConfig) Handler {
	return &handler[REQUEST, RESPONSE]{
		config:     config,
		handleFunc: f,
	}
}
