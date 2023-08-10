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
		// AuthOptOut Set this to true if the handler should skip AuthZ and AuthN.
		AuthOptOut bool
		// AuthZValidator see AuthZValidatorFn
		AuthZValidator AuthZValidatorFn
		// AuthZValidatorExtended see AuthZValidatorV2Fn
		AuthZValidatorExtended AuthZValidatorV2Fn
		// Label Optional label(name) of the handler
		Label string
		// beforeRequestValidate optional function which is given pointers to all request arguments, so they can be combined just before final validation - i.e.
		// our typical scenarios - request's payload is extended with orgId provided as path parameter. stuffing that into the actual payload may be required for the validation
		// to pass (i.e. orgId must be supplied and must be uuid type)
		beforeRequestValidate beforeRequestValidateFn
		// responseProcessors - optional collection of response processors
		responseProcessors []ResponseProcessorFn
	}

	// AuthZValidatorFn a function that takes the authenticated principal and returns whether the principal is authorized.
	// return true if the user is authorized
	// return false if the user is NOT authorized and a string indicated the reason.
	AuthZValidatorFn func(p *iam.ArmoryCloudPrincipal) (string, bool)

	// AuthZValidatorV2Fn a function that takes the authenticated principal and passes current context, so you can make auth
	// decision based on the additional conditions (i.e. headers, path parameters, etc. )
	// return true if the user is authorized
	// return false if the user is NOT authorized
	AuthZValidatorV2Fn func(ctx context.Context, p *iam.ArmoryCloudPrincipal) (string, bool)

	// HandlerArgument represents an interface of the generic argument passed into your handler method. The argument represents
	// a strongly typed struct with the data obtained from http's request path parameters, query parameters or headers. There is
	// also utility implementation ArmoryPrincipalArgument which provides access to entity issuing current request.
	// The argument's fields allow typical validation annotations - should they fail - 400 status code with details will be reported back.
	HandlerArgument interface {
		Source() ArgumentDataSource
	}

	beforeRequestValidateFn func(ctx context.Context)

	handler[T, U any] struct {
		config          HandlerConfig
		extractArgsFunc extractRequestArgumentsDelegate[T]
		handleFunc      handleRequestDelegate[T, U]
	}

	handleRequestDelegate[T, U any]        func(ctx context.Context, request T) (*Response[U], serr.Error)
	extractRequestArgumentsDelegate[T any] func(ctx context.Context, request *T, validate *validator.Validate) (interface{}, serr.Error)

	ArmoryPrincipalArgument struct {
		*iam.ArmoryCloudPrincipal
	}

	voidArgument struct{}

	ArgumentDataSource int

	Handler1Extensions[REQUEST, RESPONSE any] struct {
		*handler[REQUEST, RESPONSE]
	}
	Handler2Extensions[REQUEST, RESPONSE any, ARG HandlerArgument] struct {
		*handler[REQUEST, RESPONSE]
	}
	Handler3Extensions[REQUEST, RESPONSE any, ARG1, ARG2 HandlerArgument] struct {
		*handler[REQUEST, RESPONSE]
	}
	Handler4Extensions[REQUEST, RESPONSE any, ARG1, ARG2, ARG3 HandlerArgument] struct {
		*handler[REQUEST, RESPONSE]
	}
)

const (
	voidArgumentSource  ArgumentDataSource = -1
	PathContextSource   ArgumentDataSource = 0
	QueryContextSource  ArgumentDataSource = 1
	HeaderContextSource ArgumentDataSource = 2
	authContextSource   ArgumentDataSource = 200
)

func (r *handler[REQUEST, RESPONSE]) Config() HandlerConfig {
	return r.config
}

func (r *handler[REQUEST, RESPONSE]) GetGinHandlerFn(log *zap.SugaredLogger, requestValidator *validator.Validate, config *handlerDTO) gin.HandlerFunc {
	extensionPoints := HandlerExtensionPoints{
		BeforeRequestValidate: r.config.beforeRequestValidate,
	}
	return ginHOF(r.handleFunc, r.extractArgsFunc, config, requestValidator, &extensionPoints, log)
}

func (ArmoryPrincipalArgument) Source() ArgumentDataSource {
	return authContextSource
}

func (voidArgument) Source() ArgumentDataSource {
	return voidArgumentSource
}

// NewHandler creates a Handler from a handler function and server.HandlerConfig accepting single, strongly typed argument extracted from submitted HTTP body - typical usage for POST or PUT requests
// i.e. handler function like func OnRequest(ctx context.Context, body api.YourRequestType) (*Response[api.YourResponseType, serr.Error)
func NewHandler[REQUEST, RESPONSE any](f func(ctx context.Context, request REQUEST) (*Response[RESPONSE], serr.Error), config HandlerConfig) *Handler1Extensions[REQUEST, RESPONSE] {
	return &Handler1Extensions[REQUEST, RESPONSE]{
		&handler[REQUEST, RESPONSE]{
			config:          config,
			extractArgsFunc: extractArgsFromRequest1[REQUEST],
			handleFunc:      f,
		},
	}
}

// NewNoContentHandler - creates a Handler from a handler function and server.HandlerConfig without any input argument - typical use case would be any GET or DELETE request, where no Http content is expected
// i.e. handler function like func OnRequest(ctx context.Context) (*Response[api.YourResponseType, serr.Error)
func NewNoContentHandler[RESPONSE any](f func(ctx context.Context) (*Response[RESPONSE], serr.Error), config HandlerConfig) *Handler1Extensions[Void, RESPONSE] {
	return &Handler1Extensions[Void, RESPONSE]{
		&handler[Void, RESPONSE]{
			config:          config,
			extractArgsFunc: extractArgsFromRequest1[Void],
			handleFunc:      func(c context.Context, _ Void) (*Response[RESPONSE], serr.Error) { return f(c) },
		},
	}
}

// New1ArgHandler - creates a Handler from a handler function and server.HandlerConfig with a single input argument from submitted request body (typically Http POST or PUT) and additional argument provided by one of Path parameters, Query parameters or Headers
// i.e. handler function like func OnRequest(ctx context.Context, body api.YourRequestType, args api.AdditionalParameters) (*Response[api.YourResponseType], serr.Error)
// where api.AdditionalParameters would contain extracted values from:
//    path: i.e. /api/v1/parent/:ID/child/:childID
//    query: i.e. /api/v1/parent?foo=BAR
//    headers
func New1ArgHandler[REQUEST, RESPONSE any, CTX HandlerArgument](f func(ctx context.Context, request REQUEST, arg1 CTX) (*Response[RESPONSE], serr.Error), config HandlerConfig) *Handler2Extensions[REQUEST, RESPONSE, CTX] {

	var delegate handleRequestDelegate[REQUEST, RESPONSE] = func(ctx context.Context, r REQUEST) (*Response[RESPONSE], serr.Error) {
		args := referenceArguments[REQUEST, CTX, voidArgument, voidArgument](ctx)
		return f(ctx, r, *args.Arg1)
	}

	return &Handler2Extensions[REQUEST, RESPONSE, CTX]{
		&handler[REQUEST, RESPONSE]{
			config:          config,
			extractArgsFunc: extractArgsFromRequest2[REQUEST, CTX],
			handleFunc:      delegate,
		},
	}
}

// New1ArgNoContentHandler - creates a Handler from a handler function and server.HandlerConfig without body argument (typically Http GET or DELETE) and argument provided by one of Path parameters, Query parameters or Headers
// i.e. handler function like func OnRequest(ctx context.Context, args api.AdditionalParameters) (*Response[api.YourResponseType], serr.Error)
// where api.AdditionalParameters would contain extracted values from:
//    path: i.e. /api/v1/parent/:ID/child/:childID
//    query: i.e. /api/v1/parent?foo=BAR
//    headers
func New1ArgNoContentHandler[RESPONSE any, CTX HandlerArgument](f func(ctx context.Context, arg1 CTX) (*Response[RESPONSE], serr.Error), config HandlerConfig) *Handler2Extensions[Void, RESPONSE, CTX] {

	var delegate handleRequestDelegate[Void, RESPONSE] = func(ctx context.Context, _ Void) (*Response[RESPONSE], serr.Error) {
		args := referenceArguments[Void, CTX, voidArgument, voidArgument](ctx)
		return f(ctx, *args.Arg1)
	}

	return &Handler2Extensions[Void, RESPONSE, CTX]{
		&handler[Void, RESPONSE]{
			config:          config,
			extractArgsFunc: extractArgsFromRequest2[Void, CTX],
			handleFunc:      delegate,
		},
	}
}

// New2ArgHandler - creates a Handler from a handler function and server.HandlerConfig with body argument (typically Http POST or PUT) and 2 arguments provided by one of Path parameters, Query parameters or Headers
// i.e. handler function like func OnRequest(ctx context.Context, body api.YourRequestType, args1 api.AdditionalParameters1, args2 api.AdditionalParameters2) (*Response[api.YourResponseType], serr.Error)
// where api.AdditionalParameters1 and api.AdditionalParameters2 would contain extracted values from:
//    path: i.e. /api/v1/parent/:ID/child/:childID
//    query: i.e. /api/v1/parent?foo=BAR
//    headers
func New2ArgHandler[REQUEST, RESPONSE any, CTX1 HandlerArgument, CTX2 HandlerArgument](f func(ctx context.Context, request REQUEST, arg1 CTX1, arg2 CTX2) (*Response[RESPONSE], serr.Error), config HandlerConfig) *Handler3Extensions[REQUEST, RESPONSE, CTX1, CTX2] {

	var delegate handleRequestDelegate[REQUEST, RESPONSE] = func(ctx context.Context, r REQUEST) (*Response[RESPONSE], serr.Error) {
		args := referenceArguments[REQUEST, CTX1, CTX2, voidArgument](ctx)
		return f(ctx, r, *args.Arg1, *args.Arg2)
	}

	return &Handler3Extensions[REQUEST, RESPONSE, CTX1, CTX2]{
		&handler[REQUEST, RESPONSE]{
			config:          config,
			extractArgsFunc: extractArgsFromRequest3[REQUEST, CTX1, CTX2],
			handleFunc:      delegate,
		},
	}
}

// New2ArgNoContentHandler - creates a Handler from a handler function and server.HandlerConfig without body argument (typically Http GET or DELETE) and argument provided by one of Path parameters, Query parameters or Headers
// i.e. handler function like func OnRequest(ctx context.Context, args1 api.AdditionalParameters1, args2 api.AdditionalParameters2) (*Response[api.YourResponseType], serr.Error)
// where api.AdditionalParameters1 and api.AdditionalParameters2 would contain extracted values from:
//    path: i.e. /api/v1/parent/:ID/child/:childID
//    query: i.e. /api/v1/parent?foo=BAR
//    headers
func New2ArgNoContentHandler[RESPONSE any, CTX1 HandlerArgument, CTX2 HandlerArgument](f func(ctx context.Context, arg1 CTX1, arg2 CTX2) (*Response[RESPONSE], serr.Error), config HandlerConfig) *Handler3Extensions[Void, RESPONSE, CTX1, CTX2] {

	var delegate handleRequestDelegate[Void, RESPONSE] = func(ctx context.Context, _ Void) (*Response[RESPONSE], serr.Error) {
		args := referenceArguments[Void, CTX1, CTX2, voidArgument](ctx)
		return f(ctx, *args.Arg1, *args.Arg2)
	}

	return &Handler3Extensions[Void, RESPONSE, CTX1, CTX2]{
		&handler[Void, RESPONSE]{
			config:          config,
			extractArgsFunc: extractArgsFromRequest3[Void, CTX1, CTX2],
			handleFunc:      delegate,
		},
	}
}

// New3ArgHandler - creates a Handler from a handler function and server.HandlerConfig with body argument (typically Http POST or PUT) and 3 arguments provided by one of Path parameters, Query parameters or Headers
// i.e. handler function like func OnRequest(ctx context.Context, body api.YourRequestType, args1 api.AdditionalParameters1, args2 api.AdditionalParameters2, args3 api.AdditionalParameters3) (*Response[api.YourResponseType], serr.Error)
// where api.AdditionalParameters1, api.AdditionalParameters2 and api.AdditionalParameters3 would contain extracted values from:
//    path: i.e. /api/v1/parent/:ID/child/:childID
//    query: i.e. /api/v1/parent?foo=BAR
//    headers
func New3ArgHandler[REQUEST, RESPONSE any, CTX1 HandlerArgument, CTX2 HandlerArgument, CTX3 HandlerArgument](
  f func(ctx context.Context, request REQUEST, arg1 CTX1, arg2 CTX2, arg3 CTX3) (*Response[RESPONSE], serr.Error), config HandlerConfig) *Handler4Extensions[REQUEST, RESPONSE, CTX1, CTX2, CTX3] {

	var delegate handleRequestDelegate[REQUEST, RESPONSE] = func(ctx context.Context, r REQUEST) (*Response[RESPONSE], serr.Error) {
		args := referenceArguments[REQUEST, CTX1, CTX2, CTX3](ctx)
		return f(ctx, r, *args.Arg1, *args.Arg2, *args.Arg3)
	}

	return &Handler4Extensions[REQUEST, RESPONSE, CTX1, CTX2, CTX3]{
		&handler[REQUEST, RESPONSE]{
			config:          config,
			extractArgsFunc: extractArgsFromRequest4[REQUEST, CTX1, CTX2, CTX3],
			handleFunc:      delegate,
		},
	}
}

// New3ArgNoContentHandler - creates a Handler from a handler function and server.HandlerConfig without body argument (typically Http GET or DELETE) and 3 arguments provided by one of Path parameters, Query parameters or Headers
// i.e. handler function like func OnRequest(ctx context.Context, args1 api.AdditionalParameters1, args2 api.AdditionalParameters2, args3 api.AdditionalParameters3) (*Response[api.YourResponseType], serr.Error)
// where api.AdditionalParameters1, api.AdditionalParameters2 and api.AdditionalParameters3 would contain extracted values from:
//    path: i.e. /api/v1/parent/:ID/child/:childID
//    query: i.e. /api/v1/parent?foo=BAR
//    headers
func New3ArgNoContentHandler[RESPONSE any, CTX1 HandlerArgument, CTX2 HandlerArgument, CTX3 HandlerArgument](
  f func(ctx context.Context, arg1 CTX1, arg2 CTX2, arg3 CTX3) (*Response[RESPONSE], serr.Error), config HandlerConfig) *Handler4Extensions[Void, RESPONSE, CTX1, CTX2, CTX3] {

	var delegate handleRequestDelegate[Void, RESPONSE] = func(ctx context.Context, _ Void) (*Response[RESPONSE], serr.Error) {
		args := referenceArguments[Void, CTX1, CTX2, CTX3](ctx)
		return f(ctx, *args.Arg1, *args.Arg2, *args.Arg3)
	}

	return &Handler4Extensions[Void, RESPONSE, CTX1, CTX2, CTX3]{
		&handler[Void, RESPONSE]{
			config:          config,
			extractArgsFunc: extractArgsFromRequest4[Void, CTX1, CTX2, CTX3],
			handleFunc:      delegate,
		},
	}
}

func (r *Handler1Extensions[REQUEST, RESPONSE]) RegisterBeforeValidationHandler(beforeValidation func(body *REQUEST)) *Handler1Extensions[REQUEST, RESPONSE] {
	r.config.beforeRequestValidate = func(ctx context.Context) {
		args := referenceArguments[REQUEST, voidArgument, voidArgument, voidArgument](ctx)
		beforeValidation(args.Request)
	}
	return r
}

func (r *Handler2Extensions[REQUEST, RESPONSE, ARG]) RegisterBeforeValidationHandler(beforeValidation func(body *REQUEST, arg *ARG)) *Handler2Extensions[REQUEST, RESPONSE, ARG] {
	r.config.beforeRequestValidate = func(ctx context.Context) {
		args := referenceArguments[REQUEST, ARG, voidArgument, voidArgument](ctx)
		beforeValidation(args.Request, args.Arg1)
	}
	return r
}

func (r *Handler3Extensions[REQUEST, RESPONSE, ARG1, ARG2]) RegisterBeforeValidationHandler(beforeValidation func(body *REQUEST, arg1 *ARG1, arg2 *ARG2)) *Handler3Extensions[REQUEST, RESPONSE, ARG1, ARG2] {
	r.config.beforeRequestValidate = func(ctx context.Context) {
		args := referenceArguments[REQUEST, ARG1, ARG2, voidArgument](ctx)
		beforeValidation(args.Request, args.Arg1, args.Arg2)
	}
	return r
}

func (r *Handler4Extensions[REQUEST, RESPONSE, ARG1, ARG2, ARG3]) RegisterBeforeValidationHandler(beforeValidation func(body *REQUEST, arg1 *ARG1, arg2 *ARG2, arg3 *ARG3)) *Handler4Extensions[REQUEST, RESPONSE, ARG1, ARG2, ARG3] {
	r.config.beforeRequestValidate = func(ctx context.Context) {
		args := referenceArguments[REQUEST, ARG1, ARG2, ARG3](ctx)
		beforeValidation(args.Request, args.Arg1, args.Arg2, args.Arg3)
	}
	return r
}

func (r *handler[REQUEST, RESPONSE]) RegisterResponseProcessor(processor ResponseProcessorFn) *handler[REQUEST, RESPONSE] {
	r.config.responseProcessors = append(r.config.responseProcessors, processor)
	return r
}
