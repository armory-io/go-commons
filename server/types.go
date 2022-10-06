package server

import (
	"context"
	"github.com/armory-io/go-commons/iam"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net/http"
)

type (
	// Handler The handler interface
	// Instances of this interface should only ever be created by NewRequestResponseHandler, which happens automatically during server initialization
	// The expected way that handlers are created is by creating a provider that provides an instance of Controller
	Handler interface {
		GetHigherOrderHandlerFunc(log *zap.SugaredLogger, v *validator.Validate, handler *handlerDTO) gin.HandlerFunc
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
		handleFunc func(ctx context.Context, request T) (*Response[U], Error)
	}

	// IController baseController the base IController interface, all controllers must impl this via providing an instance of Controller or ManagementController
	IController interface {
		Handlers() []Handler
	}

	// IControllerPrefix an IController can implement this interface to have all of its handler func paths prefixed with a common path partial
	IControllerPrefix interface {
		Prefix() string
	}

	// IControllerAuthZValidator an IController can implement this interface to apply a common AuthZ validator to all exported handlers
	IControllerAuthZValidator interface {
		AuthZValidator(p *iam.ArmoryCloudPrincipal) (string, bool)
	}

	// Controller the expected way of defining endpoint collections for an Armory application
	// See the bellow example and IController, IControllerPrefix, IControllerAuthZValidator for options
	//
	// EX:
	// 	package controllers
	//
	// 	import (
	// 		"context"
	// 		"github.com/armory-io/go-commons/server"
	// 		"github.com/armory-io/sccp/internal/sccp/k8s"
	// 		"go.uber.org/zap"
	// 		"net/http"
	// 	)
	//
	// 	type ClusterController struct {
	// 		log *zap.SugaredLogger
	// 		k8s *k8s.Service
	// 	}
	//
	// 	func NewClusterController(
	// 		log *zap.SugaredLogger,
	// 		k8sService *k8s.Service,
	// 	) server.Controller {
	// 		return server.Controller{
	// 			Controller: &ClusterController{
	// 				log: log,
	// 				k8s: k8sService,
	// 			},
	// 		}
	// 	}
	//
	// 	func (c *ClusterController) Prefix() string {
	// 		return "/clusters"
	// 	}
	//
	// 	func (c *ClusterController) Handlers() []server.Handler {
	// 		return []server.Handler{
	// 			server.NewRequestResponseHandler(c.createClusterHandler, server.HandlerConfig{
	// 				Method: http.MethodPost,
	// 			}),
	// 		}
	// 	}
	//
	// 	type (
	// 		createClusterRequest struct {
	// 			AgentIdentifier string `json:"agentIdentifier" validate:"required,min=3,max=255"`
	// 			ClientId        string `json:"clientId" validate:"required"`
	// 			ClientSecret    string `json:"clientSecret" validate:"required"`
	// 		}
	// 		createClusterResponse struct {
	// 			ClusterId string `json:"clusterId"`
	// 		}
	// 	)
	//
	// 	func (c *ClusterController) createClusterHandler(
	// 		_ context.Context,
	// 		req createClusterRequest,
	// 	) (*server.Response[createClusterResponse], server.Response) {
	// 		id, err := c.k8s.CreateCluster(req.AgentIdentifier, req.ClientId, req.ClientSecret)
	//
	// 		if err != nil {
	// 			return nil, server.NewErrorResponseFromApiError(server.APIError{
	// 				Message: "Failed to create sandbox cluster",
	// 			}, server.WithCause(err))
	// 		}
	//
	// 		return server.SimpleResponse(createClusterResponse{
	// 			ClusterId: id,
	// 		}), nil
	// 	}
	Controller struct {
		fx.Out
		Controller IController `group:"server"`
	}

	controllers struct {
		fx.In
		Controllers []IController `group:"server"`
	}

	// ManagementController the same as Controller but the controllers in this group can be optionally configured
	// to run on a separate port than the server controllers
	ManagementController struct {
		fx.Out
		Controller IController `group:"management"`
	}

	managementControllers struct {
		fx.In
		Controllers []IController `group:"management"`
	}

	// Void an empty struct that can be used as a placeholder for requests/responses that do not have a body
	Void struct{}

	// RequestDetails use server.GetRequestDetailsFromContext to get this out of the request context
	RequestDetails struct {
		// Headers the headers sent along with the request
		Headers http.Header
		// QueryParameters the decoded well-formed query params from the request
		// always a non-nil map containing all the valid query parameters found
		QueryParameters map[string][]string
		// PathParameters The map of path parameters if specified in the request configuration
		// ex: path: if the path was defined as "/customer/:id" and the request was for "/customer/foo"
		// PathParameters["id"] would equal "foo"
		PathParameters map[string]string
	}

	// Response The response wrapper for a handler response to be return to the client of the request
	// StatusCode If set then it takes precedence to the default status code for the handler.
	// Headers Any values set here will be added to the response sent to the client, if Content-Type is set here then
	// 	it will override the value set in HandlerConfig.Produces
	Response[T any] struct {
		StatusCode int
		Headers    map[string][]string
		Body       T
	}
)

// SimpleResponse a convenience function for wrapping a body in a response struct with defaults
// Use this if you do not need to supply custom headers or override the handlers default status code
func SimpleResponse[T any](body T) *Response[T] {
	return &Response[T]{
		Body: body,
	}
}
