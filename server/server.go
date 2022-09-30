package server

import (
	"context"
	"encoding/json"
	"fmt"
	armoryhttp "github.com/armory-io/go-commons/http"
	"github.com/armory-io/go-commons/iam"
	"github.com/armory-io/go-commons/metadata"
	"github.com/armory-io/go-commons/metrics"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"io"
	"net/http"
	"reflect"
	"strings"
)

var sensitiveHeaderNamesInLowerCase = []string{
	"authorization",
	"x-armory-proxied-authorization",
}

func ConfigureAndStartHttpServer(
	lifecycle fx.Lifecycle,
	config armoryhttp.Configuration,
	log *zap.SugaredLogger,
	ms *metrics.Metrics,
	serverControllers Controllers,
	managementControllers ManagementControllers,
	ps *iam.ArmoryCloudPrincipalService,
	md metadata.ApplicationMetadata,
) {
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()

	// Metrics
	g.Use(metrics.GinHTTPMiddleware(ms))
	// Dist Tracing
	g.Use(otelgin.Middleware(md.Name))

	v := validator.New()
	calmLogger := log.Desugar().WithOptions(zap.AddStacktrace(zap.DPanicLevel)).Sugar()

	AuthNotRequiredGroup := g.Group("")
	AuthRequiredGroup := g.Group("")
	AuthRequiredGroup.Use(GinAuthMiddlewareV2(ps, calmLogger))

	// TODO determine whether or not management controllers should be ran on the server port or not
	// TODO ensure that metrics endpoints are here too
	// if not bootstrap new gin server
	mAuthNotRequiredGroup := AuthNotRequiredGroup
	mAuthRequiredGroup := AuthRequiredGroup

	// Wire up server controllers
	for _, c := range serverControllers.Controllers {
		registerHandlers(c, log, AuthNotRequiredGroup, AuthRequiredGroup, calmLogger, v)
	}

	// Wire up management controllers, metrics, health, info, tracing-samples, etc
	for _, c := range managementControllers.Controllers {
		registerHandlers(c, log, mAuthNotRequiredGroup, mAuthRequiredGroup, calmLogger, v)
	}

	server := armoryhttp.NewServer(config)

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := server.Start(g); err != nil {
					log.Errorf("Failed to start server: %s", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})
}

func registerHandlers(
	c controller,
	log *zap.SugaredLogger,
	AuthNotRequiredGroup *gin.RouterGroup,
	AuthRequiredGroup *gin.RouterGroup,
	calmLogger *zap.SugaredLogger,
	v *validator.Validate,
) {
	controllerConfig := &controllerConfig{}

	if c, ok := c.(ControllerPrefix); ok {
		controllerConfig.prefix = c.Prefix()
	}

	if c, ok := c.(ControllerAuthZValidator); ok {
		controllerConfig.authZValidator = c.AuthZValidator
	}

	for _, handler := range c.Handlers() {
		log.Infof("Registering REST handler %s %s", handler.Config().Method, handler.Config().Path)
		if handler.Config().AuthOptOut {
			handler.Register(AuthNotRequiredGroup, calmLogger, v, controllerConfig)
		} else {
			handler.Register(AuthRequiredGroup, calmLogger, v, controllerConfig)
		}
	}
}

func NewRequestResponseHandler[REQUEST, RESPONSE any](f func(ctx context.Context, request REQUEST) (*Response[RESPONSE], Error), config HandlerConfig) Handler {
	return &handler[REQUEST, RESPONSE]{
		config:     config,
		handleFunc: f,
	}
}

func (r *handler[REQUEST, RESPONSE]) Register(g gin.IRoutes, log *zap.SugaredLogger, v *validator.Validate, config *controllerConfig) {
	registerHandler(g, log, r.config, config, func(c *gin.Context) Error {
		response, apiErr := r.delegate(c, v)
		if apiErr != nil {
			return apiErr
		}

		statusCode := http.StatusOK
		if r.config.StatusCode != 0 {
			statusCode = r.config.StatusCode
		}
		if response.StatusCode != 0 {
			statusCode = response.StatusCode
		}
		c.Writer.WriteHeader(statusCode)
		return writeResponse(r.config, response.Body, c.Writer)
	})
}

func writeResponse(config HandlerConfig, body any, w gin.ResponseWriter) Error {
	w.Header().Set("Content-Type", config.Produces)
	switch config.Produces {
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

func (r *handler[REQUEST, RESPONSE]) Config() HandlerConfig {
	return r.config
}

func (r *handler[REQUEST, RESPONSE]) delegate(c *gin.Context, v *validator.Validate) (*Response[RESPONSE], Error) {
	method := r.config.Method
	if method == http.MethodGet || method == http.MethodDelete {
		var req REQUEST
		if err := decodeInto(extract(c), &req); err != nil {
			return nil, NewErrorResponseFromApiError(APIError{
				Message:        "Failed to extract request parameters",
				HttpStatusCode: http.StatusBadRequest,
			}, WithCause(err))
		}
		return r.handleFunc(c.Request.Context(), req)
	} else if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return nil, NewErrorResponseFromApiError(APIError{
				Message:        "Failed to read request",
				HttpStatusCode: http.StatusBadRequest,
			}, WithCause(err))
		}

		var req REQUEST
		if err := json.Unmarshal(b, &req); err != nil {
			return nil, NewErrorResponseFromApiError(APIError{
				Message:        "Failed to unmarshal request",
				HttpStatusCode: http.StatusBadRequest,
			}, WithCause(err))
		}

		if apiErr := validateRequestBody(req, v); apiErr != nil {
			return nil, apiErr
		}

		return r.handleFunc(c.Request.Context(), req)
	}

	return nil, NewErrorResponseFromApiError(APIError{
		Message:        "Method Not Allowed",
		HttpStatusCode: http.StatusMethodNotAllowed,
	})
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

func registerHandler(g gin.IRoutes, log *zap.SugaredLogger, hConfig HandlerConfig, cConfig *controllerConfig, f func(c *gin.Context) Error) {
	path := hConfig.Path
	if cConfig.prefix != "" {
		tPre := strings.TrimSuffix(cConfig.prefix, "/")
		tPath := strings.TrimPrefix(path, "/")
		path = strings.TrimSuffix(fmt.Sprintf("%s/%s",
			tPre,
			tPath,
		), "/")
		log.Debugf("Registering handler at path: %s", path)
	}
	g.Handle(hConfig.Method, path, func(c *gin.Context) {
		if apiErr := handle(c, hConfig, cConfig, f); apiErr != nil {
			WriteAndLogApiError(apiErr, c, log)
		}
	})
}

func WriteAndLogApiError(apiErr Error, c *gin.Context, log *zap.SugaredLogger) {
	errorID := uuid.NewString()
	statusCode := http.StatusInternalServerError
	if c := apiErr.Errors()[0].HttpStatusCode; c != 0 {
		statusCode = c
	}
	WriteErrorResponse(c.Writer, apiErr, statusCode, errorID, log)
	logAPIError(c, errorID, apiErr, statusCode, log)
}

func logAPIError(
	c *gin.Context,
	errorID string,
	apiErr Error,
	statusCode int,
	log *zap.SugaredLogger,
) {
	// Configure the base log fields
	fields := []any{
		"method", c.Request.Method,
		"errorID", errorID,
		"statusCode", statusCode,
	}

	// Add request headers to the logging details
	var sb strings.Builder
	for i, hKey := range maps.Keys(c.Request.Header) {
		value := "[MASKED]"
		if !slices.Contains(sensitiveHeaderNamesInLowerCase, strings.ToLower(hKey)) {
			value = strings.Join(c.Request.Header[hKey], ",")
		}
		sb.WriteString(fmt.Sprintf("%s=%s", hKey, value))
		if i+1 < len(c.Request.Header) {
			sb.WriteString(",")
		}
	}
	hVal := sb.String()
	if hVal != "" {
		fields = append(fields, "headers", hVal)
	}

	// Add the full request uri, which will include query params to logging fields
	fields = append(fields, "uri", c.Request.RequestURI)

	// If enabled add the stacktrace to the logging details
	b := apiErr.StackTraceLoggingBehavior()
	switch b {
	case DeferToDefaultBehavior:
		// By default, 4xx should *not* log stack trace. Everything else should.
		if statusCode < 400 || statusCode >= 500 {
			fields = append(fields, "stacktrace", apiErr.Stacktrace())
		}
		break
	case ForceNoStackTrace:
		break
	case ForceStackTrace:
		fields = append(fields, "stacktrace", apiErr.Stacktrace())
		break
	}

	// Add metadata about the request principal if present to the logging fields
	principal, _ := iam.ExtractPrincipalFromContext(c.Request.Context())
	if principal != nil {
		fields = append(fields, "tenant", principal.Tenant())
		fields = append(fields, "principal-name", principal.Name)
		fields = append(fields, "principal-type", principal.Type)
	}

	// If a cause was supplied add it to the logging fields
	if apiErr.Cause() != nil {
		fields = append(fields, "error", apiErr.Cause().Error())
	}

	// Add any extra details to the logging fields
	for _, extraDetails := range apiErr.ExtraDetailsForLogging() {
		fields = append(fields, extraDetails.Key, extraDetails.Value)
	}

	// Set the message
	msg := "Could not handle request"
	if apiErr.Message() != "" {
		msg = apiErr.Message()
	}

	// Log it, full send!
	log.With(fields...).Error(msg)
}

func WriteErrorResponse(writer gin.ResponseWriter, apiErr Error, statusCode int, errorID string, log *zap.SugaredLogger) {
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

func handle(c *gin.Context, hConfig HandlerConfig, cConfig *controllerConfig, f func(c *gin.Context) Error) Error {
	if !hConfig.AuthOptOut {
		if err := authorizeRequest(c.Request.Context(), cConfig, hConfig); err != nil {
			return err
		}
	}
	return f(c)
}

func authorizeRequest(ctx context.Context, cConfig *controllerConfig, hConfig HandlerConfig) Error {
	var authZValidators []AuthZValidator
	if cConfig.authZValidator != nil {
		authZValidators = append(authZValidators, cConfig.authZValidator)
	}
	if hConfig.AuthZValidator != nil {
		authZValidators = append(authZValidators, hConfig.AuthZValidator)
	}

	// If the handler has not opted out of AuthN/AuthZ, extract the principal
	principal, err := iam.ExtractPrincipalFromContext(ctx)
	if err != nil {
		return NewErrorResponseFromApiError(APIError{
			Message:        "Invalid Credentials",
			HttpStatusCode: http.StatusUnauthorized,
		}, WithCause(err))
	}

	for _, authZValidator := range authZValidators {
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

func decodeInto[T any](vars map[string]any, ptr *T) error {
	ptrType := reflect.TypeOf(*ptr)

	if ptrType.Kind() == reflect.Struct {
		if err := mapstructure.WeakDecode(vars, ptr); err != nil {
			return err
		}
		return nil
	} else if ptrType.Kind() == reflect.String {
		if len(vars) == 1 {
			for _, value := range vars {
				if err := mapstructure.WeakDecode(value, ptr); err != nil {
					return err
				}
				return nil
			}
		}
		return nil
	}
	return fmt.Errorf("internal error: could not match request with Handler, unexpected Handler type")
}

func extract(c *gin.Context) map[string]any {
	extracted := make(map[string]any)

	for _, param := range c.Params {
		extracted[param.Key] = param.Value
	}

	for key, value := range c.Request.URL.Query() {
		extracted[key] = value[0]
	}
	return extracted
}
