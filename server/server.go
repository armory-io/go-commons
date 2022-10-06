package server

import (
	"context"
	"encoding/json"
	"errors"
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
	logger *zap.SugaredLogger,
	ms *metrics.Metrics,
	serverControllers controllers,
	managementControllers managementControllers,
	ps *iam.ArmoryCloudPrincipalService,
	md metadata.ApplicationMetadata,
) error {
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()

	// Metrics
	g.Use(metrics.GinHTTPMiddleware(ms))
	// Dist Tracing
	g.Use(otelgin.Middleware(md.Name))

	requestValidator := validator.New()

	authNotEnforcedGroup := g.Group("")
	authRequiredGroup := g.Group("")
	authRequiredGroup.Use(ginAuthMiddleware(ps, logger))

	handlerRegistry, err := newHandlerRegistry(logger, requestValidator, serverControllers.Controllers, managementControllers.Controllers)
	if err != nil {
		return err
	}

	if err = handlerRegistry.registerHandlers(registerHandlersInput{
		AuthRequiredGroup:    authRequiredGroup,
		AuthNotEnforcedGroup: authNotEnforcedGroup,
	}); err != nil {
		return err
	}

	server := armoryhttp.NewServer(config)

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
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

func NewRequestResponseHandler[REQUEST, RESPONSE any](f func(ctx context.Context, request REQUEST) (*Response[RESPONSE], Error), config HandlerConfig) Handler {
	return &handler[REQUEST, RESPONSE]{
		config:     config,
		handleFunc: f,
	}
}

func (r *handler[REQUEST, RESPONSE]) GetHigherOrderHandlerFunc(log *zap.SugaredLogger, requestValidator *validator.Validate, config *handlerDTO) gin.HandlerFunc {
	return createHigherOrderHandlerFunc(r.handleFunc, config, requestValidator, log)
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

func (r *handler[REQUEST, RESPONSE]) Config() HandlerConfig {
	return r.config
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

// writeAndLogApiErrorThenAbort a helper function that will take a Response and ensure that it is logged and a properly
// formatted response is returned to the requester
func writeAndLogApiErrorThenAbort(apiErr Error, c *gin.Context, log *zap.SugaredLogger) {
	errorID := uuid.NewString()
	statusCode := http.StatusInternalServerError
	if c := apiErr.Errors()[0].HttpStatusCode; c != 0 {
		statusCode = c
	}
	writeErrorResponse(c.Writer, apiErr, statusCode, errorID, log)
	logAPIError(c, errorID, apiErr, statusCode, log)
	c.Abort()
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

	if apiErr.Origin() != "" {
		fields = append(fields, "origin", apiErr.Origin())
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

func writeErrorResponse(writer gin.ResponseWriter, apiErr Error, statusCode int, errorID string, log *zap.SugaredLogger) {
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

type requestDetailsKey struct{}

func GetRequestDetailsFromContext(ctx context.Context) (*RequestDetails, error) {
	v, ok := ctx.Value(requestDetailsKey{}).(RequestDetails)
	if !ok {
		return nil, errors.New("unable to extract request details")
	}
	return &v, nil
}

// createHigherOrderHandler creates a higher order gin handler function, that wraps the IController handler function with a function that deals with the common request/response logic
func createHigherOrderHandlerFunc[REQUEST, RESPONSE any](
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
