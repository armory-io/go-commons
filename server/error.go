package server

import (
	"encoding/json"
	"fmt"
	"github.com/armory-io/go-commons/bufferpool"
	"github.com/armory-io/go-commons/iam"
	"github.com/armory-io/go-commons/stacktrace"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"net/http"
	"strconv"
	"strings"
)

const defaultErrorCode = 42

var sensitiveHeaderNamesInLowerCase = []string{
	"authorization",
	"x-armory-proxied-authorization",
}

// ResponseContract the strongly typed error contract that will be returned to the client if a request is not successful
type ResponseContract struct {
	ErrorId string                     `json:"error_id"`
	Errors  []ResponseContractErrorDTO `json:"errors"`
}

type ResponseContractErrorDTO struct {
	Message  string         `json:"message"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Code     string         `json:"code"`
}

// APIError an error that gets embedded in ResponseContract when an error response is return to the client
type APIError struct {
	// Code The business/project error code this instance represents (not to be confused with the HTTP status code which can be retrieved via HttpStatusCode).
	// This should never change for a given error so clients can rely on it and write code against it.
	Code int
	// Message the message that will be displayed the Client
	Message string
	// Metadata that about the error that will be returned to the client
	Metadata map[string]any
	// HttpStatusCode defaults to http.StatusInternalServerError if not overridden
	HttpStatusCode int
}

type KVPair struct {
	Key   string
	Value string
}

type StackTraceLoggingBehavior int

const (
	DeferToDefaultBehavior StackTraceLoggingBehavior = iota
	ForceStackTrace
	ForceNoStackTrace
)

// apiErrorResponse is the struct for holding the error data when processing error responses.
// apiErrorResponse is package private and isn't meant for direct instantiation, you'll want to create an instance using the NewErrorResponseFromApiError or NewErrorResponseFromApiErrors. e.g.
//
//	NewErrorResponseFromApiError(someError,
//		withErrorMessage("super useful message")
//	)
//
//	NewErrorResponseFromApiErrors(
//		[]error.APIError{someError,otherError},
//		withErrorMessage("super useful message")
//	)
//
// The generator functions accept many Option's that directly affect the response and what is logged, so take a close look at the available options.
type apiErrorResponse struct {
	// errors See Error.Errors
	errors []APIError
	// extraDetailsForLogging See Error.ExtraDetailsForLogging
	extraDetailsForLogging []KVPair
	// extraResponseHeaders See Error.ExtraResponseHeaders
	extraResponseHeaders []KVPair
	// stackTraceLoggingBehavior See Error.StackTraceLoggingBehavior
	stackTraceLoggingBehavior StackTraceLoggingBehavior
	// message See Error.Message
	message string
	// cause See Error.Cause
	cause error
	// stacktrace
	stacktrace string
	// origin
	origin string
}

// Error
// You'll want to create an instance using the NewErrorResponseFromApiError or NewErrorResponseFromApiErrors. e.g.
//
//	NewErrorResponseFromApiError(someError,
//		withErrorMessage("super useful message")
//	)
//
//	NewErrorResponseFromApiErrors(
//		[]error.APIError{someError,otherError},
//		withErrorMessage("super useful message")
//	)
//
// The generator functions accept many Option's that directly affect the response and what is logged, so take a close look at the available options.
type Error interface {
	// Errors The collection of APIError associated with this instance.
	Errors() []APIError
	// ExtraDetailsForLogging Any extra details you want logged when this error is handled. Will never be null, but might be empty. NOTE: This will always be a mutable list so it can be modified at any time.
	ExtraDetailsForLogging() []KVPair
	// ExtraResponseHeaders Any extra headers you want sent to the origin when this error is handled.
	ExtraResponseHeaders() []KVPair
	// StackTraceLoggingBehavior Allows users to override the default behavior (logging stack traces for 5xx errors but not 4xx errors) and instead force stack trace on/off if they want to override the default 4xx vs. 5xx decision behavior.
	StackTraceLoggingBehavior() StackTraceLoggingBehavior
	// Message The error msg for logging, this will NOT be part of the server response
	Message() string
	// Cause The cause of the API error
	Cause() error
	// Stacktrace The stacktrace of the error
	Stacktrace() string
	// Origin the origination of the API error
	Origin() string
	// ToErrorResponseContract converts the Error into a ResponseContract
	ToErrorResponseContract(errorId string) ResponseContract
}

func (c *apiErrorResponse) Errors() []APIError {
	return c.errors
}

func (c *apiErrorResponse) ExtraDetailsForLogging() []KVPair {
	return c.extraDetailsForLogging
}

func (c *apiErrorResponse) ExtraResponseHeaders() []KVPair {
	return c.extraResponseHeaders
}

func (c *apiErrorResponse) StackTraceLoggingBehavior() StackTraceLoggingBehavior {
	return c.stackTraceLoggingBehavior
}

func (c *apiErrorResponse) Message() string {
	return c.message
}

func (c *apiErrorResponse) Cause() error {
	return c.cause
}

func (c *apiErrorResponse) Stacktrace() string {
	return c.stacktrace
}

func (c *apiErrorResponse) Origin() string {
	return c.origin
}

func (c *apiErrorResponse) ToErrorResponseContract(errorId string) ResponseContract {
	var errors []ResponseContractErrorDTO

	for _, err := range c.errors {
		code := err.Code
		if code == 0 {
			code = defaultErrorCode
		}
		errors = append(errors, ResponseContractErrorDTO{
			Message:  err.Message,
			Metadata: err.Metadata,
			Code:     strconv.Itoa(code),
		})
	}

	return ResponseContract{
		ErrorId: errorId,
		Errors:  errors,
	}
}

type Option func(aE *apiErrorResponse)

// WithExtraDetailsForLogging Adds the given logging details to what will ultimately become Error.ExtraDetailsForLogging.
func WithExtraDetailsForLogging(extraDetailsForLogging ...KVPair) Option {
	return func(aE *apiErrorResponse) {
		aE.extraDetailsForLogging = append(aE.extraDetailsForLogging, extraDetailsForLogging...)
	}
}

// WithExtraResponseHeaders Adds the given response headers to what will ultimately become Error.ExtraResponseHeaders.
func WithExtraResponseHeaders(extraResponseHeaders ...KVPair) Option {
	return func(aE *apiErrorResponse) {
		aE.extraResponseHeaders = append(aE.extraResponseHeaders, extraResponseHeaders...)
	}
}

// WithErrorMessage The error message for logging, this will NOT be part of the server response.
// Could be used as context for what went wrong if the API errors aren't self-explanatory.
// Will ultimately become Error.Message.
func WithErrorMessage(message string) Option {
	return func(aE *apiErrorResponse) {
		aE.message = message
	}
}

// WithCause The given error will be stored and used in logging and ultimately become Error.Cause.
func WithCause(err error) Option {
	return func(aE *apiErrorResponse) {
		aE.cause = err
	}
}

// WithStackTraceLoggingBehavior Sets the given StackTraceLoggingBehavior for what will ultimately become Error.StackTraceLoggingBehavior.
func WithStackTraceLoggingBehavior(behavior StackTraceLoggingBehavior) Option {
	return func(aE *apiErrorResponse) {
		aE.stackTraceLoggingBehavior = behavior
	}
}

// NewErrorResponseFromApiError Given a Single APIError and the given Option's returns an instance of Error
func NewErrorResponseFromApiError(error APIError, opts ...Option) Error {
	return NewErrorResponseFromApiErrors([]APIError{error}, opts...)
}

// NewErrorResponseFromApiErrors Given multiple APIError's and the given Option's returns an instance of Error
func NewErrorResponseFromApiErrors(errors []APIError, opts ...Option) Error {
	// get the stacktrace and caller for the error, so it can be logged
	// Ported from zap
	stack := stacktrace.Capture(2, stacktrace.Full)
	defer stack.Free()
	stackBuffer := bufferpool.Get()
	defer stackBuffer.Free()
	origin := ""
	sTrace := ""
	if stack.Count() != 0 {
		frame, more := stack.Next()

		caller := zapcore.EntryCaller{
			Defined:  frame.PC != 0,
			PC:       frame.PC,
			File:     frame.File,
			Line:     frame.Line,
			Function: frame.Function,
		}
		origin = caller.TrimmedPath()
		stackfmt := stacktrace.NewStackFormatter(stackBuffer)
		// We've already extracted the first frame, so format that
		// separately and defer to stackfmt for the rest.
		stackfmt.FormatFrame(frame)
		if more {
			stackfmt.FormatStack(stack)
		}
		sTrace = stackBuffer.String()
	}

	aec := &apiErrorResponse{
		stackTraceLoggingBehavior: DeferToDefaultBehavior,
		stacktrace:                sTrace,
		errors:                    errors,
		origin:                    origin,
	}
	for _, option := range opts {
		option(aec)
	}

	return aec
}

// writeAndLogApiErrorThenAbort a helper function that will take a Response and ensure that it is logged and a properly
// formatted response is returned to the requester
// TODO context first
func writeAndLogApiErrorThenAbort(apiErr Error, c *gin.Context, log *zap.SugaredLogger) {
	errorID := uuid.NewString()
	statusCode := http.StatusInternalServerError
	if c := apiErr.Errors()[0].HttpStatusCode; c != 0 {
		statusCode = c
	}

	span := trace.SpanFromContext(c.Request.Context())
	traceId := span.SpanContext().TraceID().String()
	spanId := span.SpanContext().SpanID().String()

	writeErrorResponse(c.Writer, apiErr, statusCode, errorID, log)
	LogAPIError(c.Request, errorID, apiErr, statusCode, traceId, spanId, log)
	c.Abort()
}

func LogAPIError(
	request *http.Request,
	errorID string,
	apiErr Error,
	statusCode int,
	traceId string,
	spanId string,
	log *zap.SugaredLogger,
) {
	// Configure the base log fields
	fields := []any{
		"method", request.Method,
		"errorID", errorID,
		"statusCode", statusCode,
	}

	if traceId != "" {
		fields = append(fields, "traceId", traceId)
	}

	if spanId != "" {
		fields = append(fields, "spanId", spanId)
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
		fields = append(fields, "src", apiErr.Origin())
	}

	// Add metadata about the request principal if present to the logging fields
	principal, _ := iam.ExtractPrincipalFromContext(request.Context())
	if principal != nil {
		fields = append(fields, "tenant", principal.Tenant())
		fields = append(fields, "principal-name", principal.Name)
		fields = append(fields, "principal-type", principal.Type)
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
