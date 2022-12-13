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
		// AuthOptOut Set this to true if the handler should skip AuthZ and AuthN, this will cause the principal to be nil in the request context
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

	beforeRequestValidateFn func(ctx context.Context)

	handler[T, U any] struct {
		config          HandlerConfig
		extractArgsFunc extractRequestArgumentsDelegate[T]
		handleFunc      handleRequestDelegate[T, U]
	}

	handleRequestDelegate[T, U any]        func(ctx context.Context, request T) (*Response[U], serr.Error)
	extractRequestArgumentsDelegate[T any] func(ctx context.Context, request *T) (interface{}, serr.Error)

	HandlerArgument interface {
		Source() ArgumentDataSource
	}

	ValidatableHandlerArgument interface {
		HandlerArgument
		Check() bool
	}

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
	PathContextSource                      = 0
	QueryContextSource                     = 1
	HeaderContextSource                    = 2
	AuthContextSource                      = 20
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
	return AuthContextSource
}

func (voidArgument) Source() ArgumentDataSource {
	return voidArgumentSource
}

// NewHandler creates a Handler from a handler function and server.HandlerConfig
func NewHandler[REQUEST, RESPONSE any](f func(ctx context.Context, request REQUEST) (*Response[RESPONSE], serr.Error), config HandlerConfig) *Handler1Extensions[REQUEST, RESPONSE] {
	return &Handler1Extensions[REQUEST, RESPONSE]{
		&handler[REQUEST, RESPONSE]{
			config:          config,
			extractArgsFunc: extractArgsFromRequest1[REQUEST],
			handleFunc:      f,
		},
	}
}

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
