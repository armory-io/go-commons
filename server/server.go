package server

import (
	"context"
	"encoding/json"
	"errors"
	armoryhttp "github.com/armory-io/go-commons/http"
	"github.com/armory-io/go-commons/iam"
	"github.com/armory-io/go-commons/metadata"
	"github.com/armory-io/go-commons/metrics"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"io"
	"net/http"
	"reflect"
)

type (
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
	// 			server.NewHandler(c.createClusterHandler, server.HandlerConfig{
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

func ConfigureAndStartHttpServer(
	lc fx.Lifecycle,
	config Configuration,
	logger *zap.SugaredLogger,
	ms *metrics.Metrics,
	serverControllers controllers,
	managementControllers managementControllers,
	ps *iam.ArmoryCloudPrincipalService,
	md metadata.ApplicationMetadata,
) error {
	gin.SetMode(gin.ReleaseMode)

	if config.Management.Port == 0 {
		var controllers []IController
		controllers = append(controllers, serverControllers.Controllers...)
		controllers = append(controllers, managementControllers.Controllers...)
		err := configureServer("http + management", lc, config.HTTP, ps, logger, ms, md, controllers...)
		if err != nil {
			return err
		}
		return nil
	}

	err := configureServer("http", lc, config.HTTP, ps, logger, ms, md, serverControllers.Controllers...)
	if err != nil {
		return err
	}
	err = configureServer("management", lc, config.Management, ps, logger, ms, md, managementControllers.Controllers...)
	if err != nil {
		return err
	}
	return nil
}

func configureServer(
	name string,
	lc fx.Lifecycle,
	httpConfig armoryhttp.HTTP,
	ps *iam.ArmoryCloudPrincipalService,
	logger *zap.SugaredLogger,
	ms *metrics.Metrics,
	md metadata.ApplicationMetadata,
	controllers ...IController,
) error {
	g := gin.New()

	// Metrics
	g.Use(metrics.GinHTTPMiddleware(ms))
	// Dist Tracing
	g.Use(otelgin.Middleware(md.Name))

	requestValidator := validator.New()

	authNotEnforcedGroup := g.Group("")
	authRequiredGroup := g.Group("")
	authRequiredGroup.Use(ginAuthMiddleware(ps, logger))

	handlerRegistry, err := newHandlerRegistry(logger, requestValidator, controllers)
	if err != nil {
		return err
	}

	if err = handlerRegistry.registerHandlers(registerHandlersInput{
		AuthRequiredGroup:    authRequiredGroup,
		AuthNotEnforcedGroup: authNotEnforcedGroup,
	}); err != nil {
		return err
	}

	server := armoryhttp.NewServer(armoryhttp.Configuration{HTTP: httpConfig})

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Infof("Starting %s server at: h: %s, p: %d, ssl: %t", name, httpConfig.Host, httpConfig.Port, httpConfig.SSL.Enabled)
			go func() {
				if err := server.Start(g); err != nil {
					logger.Fatalf("Failed to start server: %s", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})

	return nil
}

func writeResponse(contentType string, body any, w gin.ResponseWriter) Error {
	w.Header().Set("Content-Type", contentType)
	switch contentType {
	case "plain/text":
		t := reflect.TypeOf(body)
		if t.Kind() != reflect.String {
			return NewErrorResponseFromApiError(APIError{
				Message:        "Failed to write response",
				HttpStatusCode: http.StatusInternalServerError,
			},
				WithErrorMessage("Handler specified that it produces plain/text but didn't return a string as the response"),
				WithExtraDetailsForLogging(KVPair{
					Key:   "actualType",
					Value: t.String(),
				}),
			)
		}
		if _, err := w.Write([]byte(body.(string))); err != nil {
			return NewErrorResponseFromApiError(APIError{
				Message:        "Failed to write response",
				HttpStatusCode: http.StatusInternalServerError,
			}, WithCause(err))
		}
		return nil
	default:
		if err := json.NewEncoder(w).Encode(body); err != nil {
			return NewErrorResponseFromApiError(APIError{
				Message:        "Failed to write response",
				HttpStatusCode: http.StatusInternalServerError,
			}, WithCause(err))
		}
		return nil
	}
}

func validateRequestBody[T any](req T, v *validator.Validate) Error {
	err := v.Struct(req)
	if err != nil {
		vErr, ok := err.(validator.ValidationErrors)
		if ok {
			var errors []APIError
			for _, err := range vErr {
				errors = append(errors, APIError{
					Message: err.Error(),
					Metadata: map[string]any{
						"key":   err.Namespace(),
						"field": err.Field(),
						"tag":   err.Tag(),
					},
					HttpStatusCode: http.StatusBadRequest,
				})
			}
			return NewErrorResponseFromApiErrors(errors)
		}

		return NewErrorResponseFromApiError(APIError{
			Message:        "Failed to validate request",
			HttpStatusCode: http.StatusBadRequest,
		}, WithCause(err))
	}
	return nil
}

func authorizeRequest(ctx context.Context, h *handlerDTO) Error {
	// If the handler has not opted out of AuthN/AuthZ, extract the principal
	principal, err := iam.ExtractPrincipalFromContext(ctx)
	if err != nil {
		return NewErrorResponseFromApiError(APIError{
			Message:        "Invalid Credentials",
			HttpStatusCode: http.StatusUnauthorized,
		}, WithCause(err))
	}

	for _, authZValidator := range h.AuthZValidators {
		// If the handler has provided an AuthZ Validation Function, execute it.
		if msg, authorized := authZValidator(principal); !authorized {
			return NewErrorResponseFromApiError(APIError{
				Message:        "Principal Not Authorized",
				HttpStatusCode: http.StatusForbidden,
			}, WithErrorMessage(msg))
		}
	}

	return nil
}

// RequestDetails use server.GetRequestDetailsFromContext to get this out of the request context
type RequestDetails struct {
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

type requestDetailsKey struct{}

// GetRequestDetailsFromContext fetches the server.RequestDetails from the context
func GetRequestDetailsFromContext(ctx context.Context) (*RequestDetails, error) {
	v, ok := ctx.Value(requestDetailsKey{}).(RequestDetails)
	if !ok {
		return nil, errors.New("unable to extract request details")
	}
	return &v, nil
}

// createGinFunctionFromHandlerFn creates a higher order gin handler function, that wraps the IController handler function with a function that deals with the common request/response logic
func createGinFunctionFromHandlerFn[REQUEST, RESPONSE any](
	handlerFn func(ctx context.Context, request REQUEST) (*Response[RESPONSE], Error),
	handler *handlerDTO,
	requestValidator *validator.Validate,
	logger *zap.SugaredLogger,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !handler.AuthOptOut {
			if err := authorizeRequest(c.Request.Context(), handler); err != nil {
				writeAndLogApiErrorThenAbort(err, c, logger)
			}
		}

		var pathParameters = make(map[string]string)
		for _, p := range c.Params {
			pathParameters[p.Key] = p.Value
		}

		// Stuff Request details into the context
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), requestDetailsKey{}, RequestDetails{
			QueryParameters: c.Request.URL.Query(),
			PathParameters:  pathParameters,
			Headers:         c.Request.Header,
		}))

		var response *Response[RESPONSE]
		var apiError Error
		switch handler.Method {
		case http.MethodGet, http.MethodDelete:
			var req REQUEST
			response, apiError = handlerFn(c.Request.Context(), req)
			break
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			b, err := io.ReadAll(c.Request.Body)
			if err != nil {
				apiError = NewErrorResponseFromApiError(APIError{
					Message:        "Failed to read request",
					HttpStatusCode: http.StatusBadRequest,
				}, WithCause(err))
				break
			}

			// TODO what if the handler doesn't need a body/object
			// handle null body
			var req REQUEST
			if err := json.Unmarshal(b, &req); err != nil {
				apiError = NewErrorResponseFromApiError(APIError{
					Message:        "Failed to unmarshal request",
					HttpStatusCode: http.StatusBadRequest,
				}, WithCause(err))
				break
			}

			if apiError = validateRequestBody(req, requestValidator); apiError != nil {
				break
			}

			response, apiError = handlerFn(c.Request.Context(), req)
			break
		default:
			apiError = NewErrorResponseFromApiError(APIError{
				Message:        "Method Not Allowed",
				HttpStatusCode: http.StatusMethodNotAllowed,
			})
			break
		}

		if apiError != nil {
			writeAndLogApiErrorThenAbort(apiError, c, logger)
			return
		}

		var r RESPONSE
		if response == nil || reflect.ValueOf(&response.Body).Elem().IsZero() {
			if reflect.TypeOf(r) != nil && reflect.TypeOf(r).String() == "server.Void" {
				statusCode := http.StatusNoContent
				c.Writer.WriteHeader(statusCode)
				return
			} else {
				writeAndLogApiErrorThenAbort(NewErrorResponseFromApiError(
					APIError{
						Message: "Internal Server Error",
					},
					WithErrorMessage("The handler returned a nil response or nil response.Body but the response type was not server.Void, your handler should return *server.Response[server.Void] if you want to have no response body, else you must return a non nil response object."),
					WithStackTraceLoggingBehavior(ForceNoStackTrace),
				), c, logger)
				return
			}
		}

		statusCode := http.StatusOK
		if handler.StatusCode != 0 {
			statusCode = handler.StatusCode
		}
		if response.StatusCode != 0 {
			statusCode = response.StatusCode
		}
		c.Writer.WriteHeader(statusCode)
		apiError = writeResponse(handler.Produces, response.Body, c.Writer)
		if apiError != nil {
			writeAndLogApiErrorThenAbort(apiError, c, logger)
			return
		}
	}
}
