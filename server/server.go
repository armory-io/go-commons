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

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	armoryhttp "github.com/armory-io/go-commons/http"
	"github.com/armory-io/go-commons/iam"
	"github.com/armory-io/go-commons/management/info"
	"github.com/armory-io/go-commons/metadata"
	"github.com/armory-io/go-commons/metrics"
	"github.com/armory-io/go-commons/server/serr"
	"github.com/creasty/defaults"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/samber/lo"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"unsafe"
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

	// ResponseProcessorFn function, executed after the handler processing is complete. It provides user a chance to get raw response bytes and allow
	// extra processing of the response before sending it back to the caller
	ResponseProcessorFn func(ctx context.Context, bytes []byte) ([]byte, serr.Error)

	// ResponseProcessorWithOrder structure wrapping response processors - if one wants to chain multiple processors, provide proper order to build the correct pipeline
	ResponseProcessorWithOrder struct {
		Order     int
		Processor ResponseProcessorFn
	}

	// IControllerPostResponseProcessor the IController can implement this interface to provide reponse processor to all exported handlers
	IControllerPostResponseProcessor interface {
		ResponseProcessors() []ResponseProcessorWithOrder
	}

	// IControllerAuthZValidator an IController can implement this interface to apply a common AuthZ validator to all exported handlers
	IControllerAuthZValidator interface {
		AuthZValidator(p *iam.ArmoryCloudPrincipal) (string, bool)
	}

	// IControllerAuthZValidatorV2 an IController can implement this interface to apply a common AuthZ validator to all exported handlers
	IControllerAuthZValidatorV2 interface {
		AuthZValidator(ctx context.Context, p *iam.ArmoryCloudPrincipal) (string, bool)
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

	// requestArgs - structure which:
	// - always contains request's body provided as Request (of type *T). Will be of Void type for GET and DELETE request, can be Void for other types in case when no request body is required
	// - optional Arg1 of whatever type you provide, otherwise - it is of voidArgument type
	// - optional Arg2 of whatever type you provide, otherwise - it is of voidArgument type
	// - optional Arg3 of whatever type you provide, otherwise - it is of voidArgument type
	// Note: HandlerArgument is an interface, which defines where to pull the data for the argument from (typical is Path, Query or Header, but there are some special cases - like ArmoryCloudPrincipal)
	requestArgs[T any, A1 HandlerArgument, A2 HandlerArgument, A3 HandlerArgument] struct {
		Request *T
		Arg1    *A1
		Arg2    *A2
		Arg3    *A3
	}

	// HandlerExtensionPoints handler flow extensibility points - register specific handlers to plug additional processing
	// in the pipeline
	HandlerExtensionPoints struct {
		BeforeRequestValidate beforeRequestValidateFn
	}

	noopAuthService struct{}

	AuthService interface {
		VerifyPrincipalAndSetContext(tokenOrRawHeader string, c *gin.Context) error
	}

	// RequestDetails use server.ExtractRequestDetailsFromContext to get this out of the request context
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
		// RequestPath the string representing requested resources i.e. /api/v1/organizations/:orgID/...
		RequestPath string
		// LoggingMetadata
		LoggingMetadata LoggingMetadata
	}

	LoggingMetadata struct {
		Logger   *zap.SugaredLogger
		Metadata map[string]string
	}

	requestDetailsKey struct{}

	requestArgumentsKey struct{}
)

var (
	byteArrayType = reflect.TypeOf([]byte(nil))
	voidType      = reflect.TypeOf(Void{})

	extractPathDetails   = func(details *RequestDetails) any { return details.PathParameters }
	extractQueryDetails  = func(details *RequestDetails) any { return details.QueryParameters }
	extractHeaderDetails = func(details *RequestDetails) any { return details.Headers }

	unableToExtractRequestDetails = serr.APIError{
		Message:        "Unable to extract request details",
		HttpStatusCode: http.StatusInternalServerError,
	}

	errBodyRequired = serr.APIError{
		Message:        "Failed to read request",
		HttpStatusCode: http.StatusBadRequest,
	}
	errFailedToUnmarshalRequest = serr.APIError{
		Message:        "Failed to unmarshal request",
		HttpStatusCode: http.StatusBadRequest,
	}
	errFailedToSetRequestDefaults = serr.APIError{
		Message:        "Failed to read request",
		HttpStatusCode: http.StatusInternalServerError,
	}
	errFailedToReadRequest = serr.APIError{
		Message:        "Failed to read request",
		HttpStatusCode: http.StatusBadRequest,
	}
	errMethodNotAllowed = serr.APIError{
		Message:        "Method Not Allowed",
		HttpStatusCode: http.StatusMethodNotAllowed,
	}
	errServerFailedToProduceExpectedResponse = serr.APIError{
		Message:        "Failed to Produce Response Body",
		HttpStatusCode: http.StatusInternalServerError,
	}
	errInternalServerError = serr.APIError{
		Message:        "The server was not able to handle the request",
		HttpStatusCode: http.StatusInternalServerError,
	}
	invalidCredentialsError = serr.APIError{
		Message:        "Invalid Credentials",
		HttpStatusCode: http.StatusUnauthorized,
	}
	principalNotAuthorized = serr.APIError{
		Message:        "Principal Not Authorized",
		HttpStatusCode: http.StatusForbidden,
	}

	sensitiveHeaderNamesInLowerCase = []string{
		"authorization",
		"x-armory-proxied-authorization",
	}
)

// SimpleResponse a convenience function for wrapping a body in a response struct with defaults
// Use this if you do not need to supply custom headers or override the handlers default status code
func SimpleResponse[T any](body T) *Response[T] {
	return &Response[T]{
		Body: body,
	}
}

// SimpleResponseWithStatus a convenience function for wrapping a body in a response struct with defaults
// Use this if you do not need to supply custom headers
func SimpleResponseWithStatus[T any](body T, status int) *Response[T] {
	return &Response[T]{
		Body:       body,
		StatusCode: status,
	}
}

func ConfigureAndStartHttpServer(
	lc fx.Lifecycle,
	config Configuration,
	logger *zap.SugaredLogger,
	ms metrics.MetricsSvc,
	serverControllers controllers,
	managementControllers managementControllers,
	as AuthService,
	md metadata.ApplicationMetadata,
	requestValidator *validator.Validate,
	is *info.InfoService,
) error {
	gin.SetMode(gin.ReleaseMode)

	if config.Management.Port == 0 {
		var controllers []IController
		controllers = append(controllers, serverControllers.Controllers...)
		controllers = append(controllers, managementControllers.Controllers...)
		err := configureServer("http", lc, config.HTTP, config.RequestLogging, config.SPA, config.Profile, as, logger, ms, md, is, true, requestValidator, controllers...)
		if err != nil {
			return err
		}
		return nil
	}

	err := configureServer("http", lc, config.HTTP, config.RequestLogging, config.SPA, config.Profile, as, logger, ms, md, is, false, requestValidator, serverControllers.Controllers...)
	if err != nil {
		return err
	}
	err = configureServer("management", lc, config.Management, config.RequestLogging, config.SPA, config.Profile, as, logger, ms, md, is, true, requestValidator, managementControllers.Controllers...)
	if err != nil {
		return err
	}
	return nil
}

func configureServer(
	name string,
	lc fx.Lifecycle,
	httpConfig armoryhttp.HTTP,
	requestLoggingConfig RequestLoggingConfiguration,
	spaConfig SPAConfiguration,
	profile ProfileConfiguration,
	as AuthService,
	logger *zap.SugaredLogger,
	ms metrics.MetricsSvc,
	md metadata.ApplicationMetadata,
	is *info.InfoService,
	handlesManagement bool,
	requestValidator *validator.Validate,
	controllers ...IController,
) error {
	g := gin.New()

	// Dist Tracing
	g.Use(otelgin.Middleware(md.Name))

	// Metrics
	g.Use(metrics.GinHTTPMiddleware(ms))

	// Optionally enable request logging
	if requestLoggingConfig.Enabled {
		g.Use(requestLogger(logger, requestLoggingConfig))
	}

	authNotEnforcedGroup := g.Group(httpConfig.Prefix)
	authNotEnforcedGroup.Use(ginAttemptAuthMiddleware(as))

	// Allow a web-app to serve a single page application (SPA), such as react, vue, angular, etc.
	if spaConfig.Enabled {
		g.Use(spaMiddleware(spaConfig))
	}

	authRequiredGroup := g.Group(httpConfig.Prefix)
	authRequiredGroup.Use(ginEnforceAuthMiddleware(as, logger))

	handlerRegistry, err := newHandlerRegistry(name, logger, requestValidator, controllers)
	if err != nil {
		return err
	}

	if err = handlerRegistry.registerHandlers(registerHandlersInput{
		AuthRequiredGroup:    authRequiredGroup,
		AuthNotEnforcedGroup: authNotEnforcedGroup,
	}); err != nil {
		return err
	}

	// the prom handler has a bunch of logic that I don't want to have to port, so we will not make a controller for it.
	if handlesManagement {
		authNotEnforcedGroup.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	// if this is the management server and profile is enabled turn on pprof
	if handlesManagement && profile.Enabled {
		if profile.OverridePrefix != "" {
			pprof.RouteRegister(authNotEnforcedGroup, profile.OverridePrefix)
		} else {
			pprof.RouteRegister(authNotEnforcedGroup)
		}
	}

	server := armoryhttp.NewServer(armoryhttp.Configuration{HTTP: httpConfig})

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Infof("Starting %s server at: h: %s, p: %d, ssl: %t", name, httpConfig.Host, httpConfig.Port, httpConfig.SSL.Enabled)
			go func() {
				if err := server.Start(g); err != nil {
					if !errors.Is(err, http.ErrServerClosed) {
						logger.Fatalf("Failed to start server: %s", err)
					}
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})

	is.AddInfoContributor(handlerRegistry)

	return nil
}

func NewNoopAuthService() AuthService {
	return &noopAuthService{}
}

func (a *noopAuthService) VerifyPrincipalAndSetContext(tokenOrRawHeader string, c *gin.Context) error {
	return nil
}

// AddRequestDetailsToCtx is exposed for testing and allows tests to configure the request details when testing handler functions
func AddRequestDetailsToCtx(ctx context.Context, details RequestDetails) context.Context {
	return context.WithValue(ctx, requestDetailsKey{}, details)
}

// ExtractPrincipalFromContext retrieves the principal from the context and returns a serr.Error
func ExtractPrincipalFromContext(ctx context.Context) (*iam.ArmoryCloudPrincipal, serr.Error) {
	principal, err := iam.ExtractPrincipalFromContext(ctx)
	if err != nil {
		return nil, serr.NewErrorResponseFromApiError(invalidCredentialsError, serr.WithCause(err))
	}
	return principal, nil
}

type requestDetailsContext interface {
	Value(any) any
}

// ExtractRequestDetailsFromContext fetches the server.RequestDetails from the context
func ExtractRequestDetailsFromContext(ctx requestDetailsContext) (*RequestDetails, serr.Error) {
	v, ok := ctx.Value(requestDetailsKey{}).(RequestDetails)
	if !ok {
		return nil, serr.NewErrorResponseFromApiError(unableToExtractRequestDetails)
	}
	return &v, nil
}

// ExtractPathParamsFromRequestContext accepts a type param T and attempts to map the HTTP
// request's path params into T.
func ExtractPathParamsFromRequestContext[T any](ctx context.Context) (*T, serr.Error) {
	var result T
	err := extract[T](ctx, extractPathDetails, &result)
	return &result, err
}

// ExtractQueryParamsFromRequestContext accepts a type param T and attempts to map the HTTP
// request's query params into T.
// Query parameters can be a string array, so make sure your target field definition is array type as well.
func ExtractQueryParamsFromRequestContext[T any](ctx context.Context) (*T, serr.Error) {
	var result T
	err := extract[T](ctx, extractQueryDetails, &result)
	return &result, err
}

// ExtractHeaderParamsFromRequestContext accepts a type param T and attempts to map the HTTP
// header's param into T. Use `mapstructure:"<your header name>"` to annotate field to match the header.
// Headers can be a string array, so make sure your field is array type as well.
func ExtractHeaderParamsFromRequestContext[T any](ctx context.Context) (*T, serr.Error) {
	var result T
	err := extract[T](ctx, extractHeaderDetails, &result)
	return &result, err
}

// ginHOF creates Higher Order gin Handler Function, that wraps the IController handler function with a function that deals with the common request/response logic
func ginHOF[REQUEST, RESPONSE any](
	handlerFn handleRequestDelegate[REQUEST, RESPONSE],
	extractRequestArgsFn extractRequestArgumentsDelegate[REQUEST],
	handler *handlerDTO,
	requestValidator *validator.Validate,
	extensions *HandlerExtensionPoints,
	logger *zap.SugaredLogger,
) gin.HandlerFunc {
	return func(c *gin.Context) {

		// recover from panics and return a well-formed error and log the details
		defer func() {
			if r := recover(); r != nil {
				onRequestCompleted(c, logger, r)
			}
		}()

		loggingMetadata := extractLoggingMetadata(c.Request.Context())
		onPrepareRequestContext(c, LoggingMetadata{
			Logger:   logger.With(ExtractLoggingFields(loggingMetadata)...),
			Metadata: loggingMetadata,
		})

		if !onAuthorizeRequest(c, handler, logger) {
			return
		}

		var req *REQUEST
		if r, ok := onExtractRequestBodyAndParameters(c, extractRequestArgsFn, logger, requestValidator, func(r *REQUEST) bool { return onValidateRequest(c, r, logger, requestValidator, extensions) }); !ok {
			return
		} else {
			req = r
		}

		response, apiError := handlerFn(c.Request.Context(), *req)
		if apiError != nil {
			writeAndLogApiErrorThenAbort(c, apiError, logger)
			return
		}

		onHandleResponse(c, response, logger, handler)
	}
}

func onRequestCompleted(c *gin.Context, logger *zap.SugaredLogger, panicReason any) {
	cause := fmt.Sprintf("%s", panicReason)
	if cause == "" {
		cause = "panic cause was nil"
	}
	writeAndLogApiErrorThenAbort(c, serr.NewErrorResponseFromApiError(
		errInternalServerError,
		serr.WithErrorMessage("The handler panicked"),
		serr.WithStackTraceLoggingBehavior(serr.ForceStackTrace),
		serr.WithFrameSkips(6),
		serr.WithCause(fmt.Errorf(cause)),
	), logger)
}

func onPrepareRequestContext(c *gin.Context, loggingMetadata LoggingMetadata) {
	// Stuff Request details into the context
	requestDetails := RequestDetails{
		QueryParameters: c.Request.URL.Query(),
		PathParameters:  extractPathParameters(c),
		Headers:         c.Request.Header,
		RequestPath:     c.Request.URL.Path,
		LoggingMetadata: loggingMetadata,
	}
	c.Request = c.Request.WithContext(AddRequestDetailsToCtx(c.Request.Context(), requestDetails))
}

func onAuthorizeRequest(c *gin.Context, handler *handlerDTO, logger *zap.SugaredLogger) bool {
	if !handler.AuthOptOut {
		if err := authorizeRequest(c.Request.Context(), handler); err != nil {
			writeAndLogApiErrorThenAbort(c, err, logger)
			return false
		}
	}
	return true
}

func onExtractRequestBodyAndParameters[REQUEST any](
	c *gin.Context,
	extractRequestArgsFn extractRequestArgumentsDelegate[REQUEST],
	logger *zap.SugaredLogger,
	validator *validator.Validate,
	validateHandler func(req *REQUEST) bool) (*REQUEST, bool) {

	req, shouldValidateBody, apiError := extractRequestBody[REQUEST](c)
	if apiError != nil {
		writeAndLogApiErrorThenAbort(c, apiError, logger)
		return nil, false
	}

	if nil == extractRequestArgsFn {
		extractRequestArgsFn = extractArgsFromRequest1[REQUEST]
	}

	args, apiError := extractRequestArgsFn(c.Request.Context(), req, validator)
	if apiError != nil {
		writeAndLogApiErrorThenAbort(c, apiError, logger)
		return nil, false
	}

	c.Request = c.Request.WithContext(addRequestArgumentsToCtx(c.Request.Context(), args))

	if shouldValidateBody {
		return req, validateHandler(req)
	}

	return req, true
}

func onValidateRequest[REQUEST any](c *gin.Context, req *REQUEST, logger *zap.SugaredLogger, requestValidator *validator.Validate, extensions *HandlerExtensionPoints) bool {
	if extensions.BeforeRequestValidate != nil {
		extensions.BeforeRequestValidate(c.Request.Context())
	}

	apiError := validateRequestBody(req, requestValidator)
	if nil != apiError {
		writeAndLogApiErrorThenAbort(c, apiError, logger)
		return false
	}

	if err := defaults.Set(req); err != nil {
		apiError = serr.NewErrorResponseFromApiError(errFailedToSetRequestDefaults, serr.WithCause(err))
		writeAndLogApiErrorThenAbort(c, apiError, logger)
		return false
	}

	return true
}

func onHandleResponse[RESPONSE any](c *gin.Context, response *Response[RESPONSE], logger *zap.SugaredLogger, handler *handlerDTO) {
	var r RESPONSE
	responseType := reflect.TypeOf(r)
	if response == nil || reflect.ValueOf(&response.Body).Elem().IsZero() {
		if responseType != nil && responseType == voidType {
			c.Status(http.StatusNoContent)
			_, _ = c.Writer.Write([]byte{})
			return
		} else {
			writeAndLogApiErrorThenAbort(c, serr.NewErrorResponseFromApiError(
				errServerFailedToProduceExpectedResponse,
				serr.WithErrorMessage("The handler returned a nil response or nil response.Body but the response type was not server.Void, your handler should return *server.Response[server.Void] if you want to have no response body, else you must return a non nil response object."),
				serr.WithStackTraceLoggingBehavior(serr.ForceNoStackTrace),
			), logger)
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
	c.Status(statusCode)

	for header, values := range response.Headers {
		for _, value := range values {
			c.Header(header, value)
		}
	}

	apiError := writeResponse(c.Request.Context(), handler.Produces, response.Body, c.Writer, handler.ResponseProcessors)
	if apiError != nil {
		writeAndLogApiErrorThenAbort(c, apiError, logger)
		return
	}
}

func authorizeRequest(ctx context.Context, h *handlerDTO) serr.Error {
	// If the handler has not opted out of AuthN/AuthZ, extract the principal
	principal, err := ExtractPrincipalFromContext(ctx)
	if err != nil {
		return err
	}

	for _, authZValidator := range h.AuthZValidators {
		// If the handler has provided an AuthZ Validation Function, execute it.
		if msg, authorized := authZValidator(ctx, principal); !authorized {
			return serr.NewErrorResponseFromApiError(principalNotAuthorized, serr.WithErrorMessage(msg))
		}
	}

	return nil
}

func addRequestArgumentsToCtx(ctx context.Context, arguments interface{}) context.Context {
	return context.WithValue(ctx, requestArgumentsKey{}, arguments)
}

func referenceArguments[REQUEST any, ARG1 HandlerArgument, ARG2 HandlerArgument, ARG3 HandlerArgument](ctx context.Context) requestArgs[REQUEST, ARG1, ARG2, ARG3] {
	return ctx.Value(requestArgumentsKey{}).(requestArgs[REQUEST, ARG1, ARG2, ARG3])
}

func validateRequestBody[T any](req T, v *validator.Validate) serr.Error {
	err := v.Struct(req)
	if err != nil {
		vErr, ok := err.(validator.ValidationErrors)
		if ok {
			var errs []serr.APIError
			for _, err := range vErr {
				errs = append(errs, serr.APIError{
					Message: err.Error(),
					Metadata: map[string]any{
						"key":   err.Namespace(),
						"field": err.Field(),
						"tag":   err.Tag(),
					},
					HttpStatusCode: http.StatusBadRequest,
				})
			}
			return serr.NewErrorResponseFromApiErrors(errs,
				serr.WithErrorMessage("Failed to validate request body"),
				serr.WithCause(vErr),
			)
		}

		return serr.NewErrorResponseFromApiError(serr.APIError{
			Message:        "Failed to validate request",
			HttpStatusCode: http.StatusBadRequest,
		}, serr.WithCause(err))
	}
	return nil
}

func writeResponse(ctx context.Context, contentType string, body any, w gin.ResponseWriter, processors []ResponseProcessorFn) serr.Error {
	w.Header().Set("Content-Type", contentType)
	switch contentType {
	case "text/plain", "application/yaml":
		return writeStringResponse(ctx, contentType, body, w, processors)
	case "application/octet-stream":
		return writeOctetStream(contentType, body, w)
	default:
		return writeJsonResponse(ctx, body, w, processors)
	}
}

func writeJsonResponse(ctx context.Context, body any, w gin.ResponseWriter, processors []ResponseProcessorFn) serr.Error {
	bytes, err := json.Marshal(body)
	if err != nil {
		return serr.NewErrorResponseFromApiError(serr.APIError{
			Message:        "Failed to marshal response",
			HttpStatusCode: http.StatusInternalServerError,
		}, serr.WithCause(err))
	}

	for _, processor := range processors {
		b, sErr := processor(ctx, bytes)
		if sErr != nil {
			return sErr
		}
		bytes = b
	}

	if _, err = w.Write(bytes); err != nil {
		return serr.NewErrorResponseFromApiError(serr.APIError{
			Message:        "Failed to write response",
			HttpStatusCode: http.StatusInternalServerError,
		}, serr.WithCause(err))
	}

	return nil
}

// writeOctetStream expects the body to be an io.ReadCloser, if it is, it will be copied to the response writer.
// This can probably be refactored later, if needed to allow the body to be a byte[] or Reader vs only allowing ReadCloser.
func writeOctetStream(contentType string, body any, w gin.ResponseWriter) serr.Error {
	bodyContent, ok := body.(io.ReadCloser)
	if !ok {
		return serr.NewErrorResponseFromApiError(serr.APIError{
			Message:        "Failed to write response",
			HttpStatusCode: http.StatusInternalServerError,
		},
			serr.WithErrorMessage(fmt.Sprintf("Handler specified that it produces %s but didn't return a ReadCloser as the response", contentType)),
		)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer bodyContent.Close()

	if _, err := io.Copy(w, bodyContent); err != nil {
		return serr.NewErrorResponseFromApiError(serr.APIError{
			Message:        "Failed to write response",
			HttpStatusCode: http.StatusInternalServerError,
		},
			serr.WithCause(err),
			serr.WithErrorMessage("Failed to copy handler body to response writer"),
		)
	}
	return nil
}

func writeStringResponse(ctx context.Context, contentType string, body any, w gin.ResponseWriter, processors []ResponseProcessorFn) serr.Error {
	t := reflect.TypeOf(body)
	canWrite := false
	bytes := lo.IfF(t.Kind() == reflect.String,
		func() []byte {
			canWrite = true
			return []byte(body.(string))
		}).ElseF(func() []byte {
		res, ok := body.([]byte)
		canWrite = ok
		return res
	})

	if !canWrite {
		return serr.NewErrorResponseFromApiError(serr.APIError{
			Message:        "Failed to write response",
			HttpStatusCode: http.StatusInternalServerError,
		},
			serr.WithErrorMessage(fmt.Sprintf("Handler specified that it produces %s but didn't return a string or []byte as the response", contentType)),
			serr.WithExtraDetailsForLogging(serr.KVPair{
				Key:   "actualType",
				Value: t.String(),
			}),
		)
	}

	for _, processor := range processors {
		b, sErr := processor(ctx, bytes)
		if sErr != nil {
			return sErr
		}
		bytes = b
	}

	if _, err := w.Write(bytes); err != nil {
		return serr.NewErrorResponseFromApiError(serr.APIError{
			Message:        "Failed to write response",
			HttpStatusCode: http.StatusInternalServerError,
		}, serr.WithCause(err))
	}
	return nil
}

func extract[T any](ctx context.Context, pick func(details *RequestDetails) any, target *T) serr.Error {
	d, err := ExtractRequestDetailsFromContext(ctx)
	if err != nil {
		return err
	}

	if err := mapstructure.WeakDecode(pick(d), target); err != nil {
		return serr.NewErrorResponseFromApiError(unableToExtractRequestDetails, serr.WithCause(err))
	}
	return nil
}

func extractPathParameters(c *gin.Context) map[string]string {
	var pathParameters = make(map[string]string)
	for _, p := range c.Params {
		pathParameters[p.Key] = p.Value
	}
	return pathParameters
}

func extractRequestBody[REQUEST any](c *gin.Context) (*REQUEST, bool, serr.Error) {
	var req REQUEST
	shouldProcessBody := false
	isArrayType := false

	requestType := lo.IfF(reflect.TypeOf(req) != nil, func() reflect.Type {
		t := reflect.TypeOf(req)
		isArrayType = t.Kind() == reflect.Array || t.Kind() == reflect.Slice
		return reflect.TypeOf(req)
	}).Else(voidType)

	switch c.Request.Method {
	case http.MethodGet, http.MethodDelete:
		shouldProcessBody = false
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		shouldProcessBody = requestType != voidType
	default:
		return nil, false, serr.NewErrorResponseFromApiError(errMethodNotAllowed)
	}
	if shouldProcessBody {
		shouldProcessBody = !isArrayType
		if c.Request.Body == nil {
			return nil, shouldProcessBody, serr.NewErrorResponseFromApiError(errBodyRequired)
		}
		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return nil, shouldProcessBody, serr.NewErrorResponseFromApiError(errFailedToReadRequest, serr.WithCause(err))
		}
		if requestType == byteArrayType {
			req = *(*REQUEST)(unsafe.Pointer(&b))
		} else {
			if err := json.Unmarshal(b, &req); err != nil {
				return nil, shouldProcessBody, handleUnmarshalError(b, err)
			}
		}
	}
	return &req, shouldProcessBody, nil
}

func handleUnmarshalError(bytes []byte, err error) serr.Error {
	var meta map[string]any = nil
	offset := 0

	syntaxError, ok := err.(*json.SyntaxError)
	if ok {
		meta = make(map[string]any)
		offset = int(syntaxError.Offset)
		meta["reason"] = err.Error()
	}
	unmarshalError, ok := err.(*json.UnmarshalTypeError)
	if ok {
		meta = make(map[string]any)
		offset = int(unmarshalError.Offset)
		meta["path"] = unmarshalError.Struct + lo.Ternary(unmarshalError.Struct == "" || unmarshalError.Field == "", "", ".") + unmarshalError.Field
		meta["providedType"] = unmarshalError.Value
		meta["expectedType"] = lo.Ternary(unmarshalError.Type.Name() != "", unmarshalError.Type.Name(), strconv.Itoa(int(unmarshalError.Type.Kind())))
		meta["reason"] = "cannot unmarshal data"
	}

	if nil != meta {
		meta["offset"] = offset
		line := 0
		column := 0
		for _, c := range bytes[:offset] {
			column++
			if c == '\n' {
				line++
				column = 0
			}
		}
		meta["line"] = line
		meta["column"] = column
	}

	returnErr := serr.APIError{
		Message:        errFailedToUnmarshalRequest.Message,
		Metadata:       meta,
		HttpStatusCode: errFailedToUnmarshalRequest.HttpStatusCode,
		Code:           errFailedToUnmarshalRequest.Code,
	}

	return serr.NewErrorResponseFromApiError(returnErr, serr.WithCause(err))
}

// writeAndLogApiErrorThenAbort a helper function that will take a serr.Error and ensure that it is logged and a properly
// formatted response is returned to the requester
func writeAndLogApiErrorThenAbort(c *gin.Context, apiErr serr.Error, log *zap.SugaredLogger) {
	errorID := uuid.NewString()
	statusCode := http.StatusInternalServerError
	if c := apiErr.Errors()[0].HttpStatusCode; c != 0 {
		statusCode = c
	}

	writeErrorResponse(c.Writer, apiErr, statusCode, errorID, log)
	LogAPIError(c.Request, errorID, apiErr, statusCode, log)
	c.Abort()
}

func LogAPIError(
	request *http.Request,
	errorID string,
	apiErr serr.Error,
	statusCode int,
	log *zap.SugaredLogger,
) {
	fields := getBaseFields(request, statusCode)

	fields = append(fields, "errorID", errorID)

	// If enabled add the stacktrace to the logging details
	b := apiErr.StackTraceLoggingBehavior()
	switch b {
	case serr.DeferToDefaultBehavior:
		// By default, 4xx should *not* log stack trace. Everything else should.
		if statusCode < 400 || statusCode >= 500 {
			fields = append(fields, "stacktrace", apiErr.Stacktrace())
		}
	case serr.ForceStackTrace:
		fields = append(fields, "stacktrace", apiErr.Stacktrace())
	}

	if apiErr.Origin() != "" {
		fields = append(fields, "src", apiErr.Origin())
	}

	// If a cause was supplied add it to the logging fields
	if apiErr.Cause() != nil {
		fields = append(fields, "error", apiErr.Cause())
	}

	// Add any extra details to the logging fields
	for _, extraDetails := range apiErr.ExtraDetailsForLogging() {
		fields = append(fields, extraDetails.Key, extraDetails.Value)
	}

	// Set the message
	msg := "Handler did not process request successfully"
	if apiErr.Message() != "" {
		msg = apiErr.Message()
	}

	// Log it, full send!
	log.With(fields...).Error(msg)
}

func getBaseFields(
	request *http.Request,
	statusCode int,
) []any {
	// Configure the base log fields
	fields := []any{
		"method", request.Method,
		"status", strconv.Itoa(statusCode),
	}

	// Add request headers to the logging details
	var sb strings.Builder
	for i, hKey := range maps.Keys(request.Header) {
		value := "[MASKED]"
		if !slices.Contains(sensitiveHeaderNamesInLowerCase, strings.ToLower(hKey)) {
			value = strings.Join(request.Header[hKey], ",")
		}
		sb.WriteString(fmt.Sprintf("%s=%s", hKey, value))
		if i+1 < len(request.Header) {
			sb.WriteString(",")
		}
	}
	hVal := sb.String()
	if hVal != "" {
		fields = append(fields, "headers", hVal)
	}

	// Add the full request uri, which will include query params to logging fields
	fields = append(fields, "uri", request.RequestURI)

	fields = append(fields, ExtractLoggingFields(extractLoggingMetadata(request.Context()))...)

	return fields
}

func writeErrorResponse(writer gin.ResponseWriter, apiErr serr.Error, statusCode int, errorID string, log *zap.SugaredLogger) {
	writer.Header().Set("content-type", "application/json")

	for _, header := range apiErr.ExtraResponseHeaders() {
		writer.Header().Add(header.Key, header.Value)
	}

	writer.WriteHeader(statusCode)
	err := json.NewEncoder(writer).Encode(apiErr.ToErrorResponseContract(errorID))
	if err != nil {
		log.Errorf("Failed to write error response: %s", err)
	}
}

// requestLogger ia a simple middleware that logs request details if the path isn't on the blocklist and the status range is permitted
func requestLogger(log *zap.SugaredLogger, config RequestLoggingConfiguration) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		status := c.Writer.Status()
		if status >= 200 && status < 300 && config.Disable2XX {
			return
		}

		if status >= 300 && status < 400 && config.Disable3XX {
			return
		}

		if status >= 400 && status < 400 && config.Disable4XX {
			return
		}

		if status >= 500 && status < 600 && config.Disable5XX {
			return
		}

		if !slices.Contains(config.BlockList, c.FullPath()) {
			log.
				With(getBaseFields(c.Request, c.Writer.Status())...).
				Infof("[ REQUEST LOGGER ]")
		}

	}
}

func extractLoggingMetadata(ctx context.Context) map[string]string {
	fields := map[string]string{}

	span := trace.SpanFromContext(ctx)
	traceId := span.SpanContext().TraceID().String()
	if traceId != "" {
		fields["trace.id"] = traceId
	}
	spanId := span.SpanContext().SpanID().String()
	if spanId != "" {
		fields["span.id"] = spanId
	}

	// Add metadata about the request principal if present to the logging fields
	principal, _ := iam.ExtractPrincipalFromContext(ctx)
	if principal != nil {
		fields["tenant"] = principal.Tenant()
		fields["principal-name"] = principal.Name
		fields["principal-type"] = string(principal.Type)
	}

	return fields
}

func ExtractLoggingFields(metadata map[string]string) []any {
	var fields []any
	for k, v := range metadata {
		fields = append(fields, k, v)
	}
	return fields
}

func extractHandlerArgumentFromContext[CTX HandlerArgument](c context.Context, v *validator.Validate) (*CTX, serr.Error) {
	result, err := extractHandlerArgumentFromContextInternal[CTX](c)
	if result != nil && err == nil {
		err = validateRequestBody[CTX](*result, v)
	}
	return result, err
}

func extractHandlerArgumentFromContextInternal[CTX HandlerArgument](c context.Context) (*CTX, serr.Error) {
	var arg CTX
	switch arg.Source() {

	case PathContextSource:
		err := extract(c, extractPathDetails, &arg)
		return &arg, err

	case QueryContextSource:
		err := extract(c, extractQueryDetails, &arg)
		return &arg, err

	case HeaderContextSource:
		err := extract(c, extractHeaderDetails, &arg)
		return &arg, err

	case authContextSource:
		principal, err := ExtractPrincipalFromContext(c)
		var retValue interface{} = &ArmoryPrincipalArgument{principal}
		return retValue.(*CTX), err

	case voidArgumentSource:
		var retValue interface{} = &voidArgument{}
		return retValue.(*CTX), nil
	}

	return nil, serr.NewSimpleError(fmt.Sprintf("not supported argument source %d", arg.Source()), nil)
}

func aggregateErrors(items ...serr.Error) serr.Error {
	if len(items) == 0 {
		return nil
	}

	apiErrors := lo.FlatMap(items, func(e serr.Error, _ int) []serr.APIError {
		if e != nil {
			return e.Errors()
		}
		return []serr.APIError{}
	})

	if len(apiErrors) == 0 {
		return nil
	}
	return serr.NewErrorResponseFromApiErrors(apiErrors)
}

func extractArgsFromRequest1[REQUEST any](c context.Context, r *REQUEST, _ *validator.Validate) (interface{}, serr.Error) {
	return requestArgs[REQUEST, voidArgument, voidArgument, voidArgument]{Request: r}, nil
}

func extractArgsFromRequest2[REQUEST any, ARG1 HandlerArgument](c context.Context, r *REQUEST, v *validator.Validate) (interface{}, serr.Error) {
	arg, err := extractHandlerArgumentFromContext[ARG1](c, v)
	if nil != err {
		return nil, err
	}
	return requestArgs[REQUEST, ARG1, voidArgument, voidArgument]{Request: r, Arg1: arg}, nil
}

func extractArgsFromRequest3[REQUEST any, ARG1 HandlerArgument, ARG2 HandlerArgument](c context.Context, r *REQUEST, v *validator.Validate) (interface{}, serr.Error) {
	arg1, err1 := extractHandlerArgumentFromContext[ARG1](c, v)
	arg2, err2 := extractHandlerArgumentFromContext[ARG2](c, v)
	if err := aggregateErrors(err1, err2); err != nil {
		return nil, err
	}
	return requestArgs[REQUEST, ARG1, ARG2, voidArgument]{Request: r, Arg1: arg1, Arg2: arg2}, nil
}

func extractArgsFromRequest4[REQUEST any, ARG1 HandlerArgument, ARG2 HandlerArgument, ARG3 HandlerArgument](c context.Context, r *REQUEST, v *validator.Validate) (interface{}, serr.Error) {
	arg1, err1 := extractHandlerArgumentFromContext[ARG1](c, v)
	arg2, err2 := extractHandlerArgumentFromContext[ARG2](c, v)
	arg3, err3 := extractHandlerArgumentFromContext[ARG3](c, v)

	if err := aggregateErrors(err1, err2, err3); err != nil {
		return nil, err
	}

	return requestArgs[REQUEST, ARG1, ARG2, ARG3]{Request: r, Arg1: arg1, Arg2: arg2, Arg3: arg3}, nil
}
