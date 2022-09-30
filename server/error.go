package server

const defaultErrorCode = "42"

// ErrorResponseContract the strongly typed error contract that will be returned to the client if a request is not successful
type ErrorResponseContract struct {
	ErrorId string                          `json:"error_id"`
	Errors  []ErrorResponseContractErrorDTO `json:"errors"`
}

type ErrorResponseContractErrorDTO struct {
	Message  string         `json:"message"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Code     string         `json:"code"`
}

// APIError an error that gets embedded in ErrorResponseContract when an error response is return to the client
type APIError struct {
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
	// ExtraResponseHeaders Any extra headers you want sent to the caller when this error is handled.
	ExtraResponseHeaders() []KVPair
	// StackTraceLoggingBehavior Allows users to override the default behavior (logging stack traces for 5xx errors but not 4xx errors) and instead force stack trace on/off if they want to override the default 4xx vs. 5xx decision behavior.
	StackTraceLoggingBehavior() StackTraceLoggingBehavior
	// Message The error msg for logging, this will NOT be part of the server response
	Message() string
	// Cause The cause of the API error
	Cause() error
	// Stacktrace The stacktrace of the error
	Stacktrace() string
	// ToErrorResponseContract converts the Error into a ErrorResponseContract
	ToErrorResponseContract(errorId string) ErrorResponseContract
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

func (c *apiErrorResponse) ToErrorResponseContract(errorId string) ErrorResponseContract {
	var errors []ErrorResponseContractErrorDTO

	// TODO if error code isn't set default it to defaultErrorCode
	for _, err := range c.errors {
		errors = append(errors, ErrorResponseContractErrorDTO{
			Message:  err.Message,
			Metadata: err.Metadata,
		})
	}

	return ErrorResponseContract{
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
	aec := &apiErrorResponse{
		stackTraceLoggingBehavior: DeferToDefaultBehavior,
		stacktrace:                takeStacktrace(1),
		errors:                    []APIError{error},
	}
	for _, option := range opts {
		option(aec)
	}

	return aec
}

// NewErrorResponseFromApiErrors Given multiple APIError's and the given Option's returns an instance of Error
func NewErrorResponseFromApiErrors(errors []APIError, opts ...Option) Error {
	aec := &apiErrorResponse{
		stackTraceLoggingBehavior: DeferToDefaultBehavior,
		stacktrace:                takeStacktrace(1),
		errors:                    errors,
	}
	for _, option := range opts {
		option(aec)
	}

	return aec
}
