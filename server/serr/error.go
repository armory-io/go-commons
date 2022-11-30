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

// Package serr This package contains all the logic and helper methods for creating and serializing API Errors
// This is in a separate package than server so that as a developer you get better intellisense when creating errors
// Heavily inspired from Nike Backstopper
package serr

import (
	"github.com/armory-io/go-commons/bufferpool"
	"github.com/armory-io/go-commons/stacktrace"
	"go.uber.org/zap/zapcore"
	"strconv"
)

const defaultErrorCode = 42

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

// APIError is an error that gets embedded in ResponseContract when an error response is returned to the client
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
	// frame skips
	framesToSkip int
}

// Error
// You'll want to create an instance using one of the New* functions in this package such as NewSimpleError, NewErrorResponseFromApiError, etc. e.g.
//
//	// A simple use case
//	serr.NewSimpleError("super useful message", someErrThatIsTheCauseThatWillBeLogged)
//
//	// A simple use case, where there isn't a cause
//	serr.NewSimpleError("super useful message", nil)
//
//	// A simple use case, where you want to override the default status code of 500
//	serr.NewSimpleErrorWithStatusCode("could not find the thing", http.StatusNotFound, nil)
//
//	// The kitchen sink
//	serr.NewErrorResponseFromApiError(serr.APIError{
//		Message: "Server can not produce requested content type",
//		Metadata: map[string]any{
//			"requested": accept,
//			"available": strings.Join(availableMimeTypes, ", "),
//		},
//		HttpStatusCode: http.StatusBadRequest,
//	},
//		serr.WithErrorMessage("Some extra message for the logs, that replaces the default, can't handle request message"),
//		serr.WithCause(err),
//		serr.WithExtraDetailsForLogging(
//			serr.KVPair{
//				Key:   "requested-type",
//				Value: accept,
//			},
//			serr.KVPair{
//				Key:   "available-types",
//				Value: strings.Join(availableMimeTypes, ", "),
//			},
//		))
//		serr.WithExtraResponseHeaders(serr.KVPair{
//			Key:   "X-Armory-Custom-header",
//			Value: "custom header value",
//		})
//		serr.WithStackTraceLoggingBehavior(serr.ForceStackTrace) // tweak the stacktrace behavior
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

// WithFrameSkips Overrides the number of frames to skip when generating the stack trace.
func WithFrameSkips(framesToSkip int) Option {
	return func(aE *apiErrorResponse) {
		aE.framesToSkip = framesToSkip
	}
}

// NewErrorResponseFromApiError Given a Single APIError and the given Option's returns an instance of Error
func NewErrorResponseFromApiError(error APIError, opts ...Option) Error {
	return NewErrorResponseFromApiErrors([]APIError{error}, opts...)
}

// NewErrorResponseFromApiErrors Given multiple APIError's and the given Option's returns an instance of Error
func NewErrorResponseFromApiErrors(errors []APIError, opts ...Option) Error {
	aec := &apiErrorResponse{
		stackTraceLoggingBehavior: DeferToDefaultBehavior,
		errors:                    errors,
		framesToSkip:              2,
	}
	for _, option := range opts {
		option(aec)
	}

	// get the stacktrace and caller for the error, so it can be logged
	// Ported from zap
	stack := stacktrace.Capture(aec.framesToSkip, stacktrace.Full)
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

	aec.origin = origin
	aec.stacktrace = sTrace

	return aec
}

// NewSimpleError is a helper function that will create a serr.Error that satisfies most use cases.
//
// Defaults to http.StatusInternalServerError (500) status code.
//
// Use serr.NewSimpleErrorWithStatusCode to override the status code.
func NewSimpleError(msgForResponse string, cause error) Error {
	return NewErrorResponseFromApiError(APIError{
		Message: msgForResponse,
	}, WithCause(cause))
}

// NewSimpleErrorWithStatusCode is a helper function that will create a serr.Error that satisfies most use cases
func NewSimpleErrorWithStatusCode(msgForResponse string, statusCodeForResponse int, cause error) Error {
	return NewErrorResponseFromApiError(APIError{
		Message:        msgForResponse,
		HttpStatusCode: statusCodeForResponse,
	}, WithCause(cause))
}

// NewWrappedErrorWithStatusCode is a helper function that creates a serr.Error from an error. It is appropriate
// to use when the provided err is intended for end-user consumption (e.g., a custom error message).
func NewWrappedErrorWithStatusCode(err error, statusCodeForResponse int) Error {
	return NewSimpleErrorWithStatusCode(err.Error(), statusCodeForResponse, err)
}
