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
	"fmt"
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
	}

	// AuthZValidatorFn a function that takes the authenticated principal and returns whether the principal is authorized.
	// return true if the user is authorized
	// return false if the user is NOT authorized and a string indicated the reason.
	AuthZValidatorFn func(p *iam.ArmoryCloudPrincipal) (string, bool)

	handler[T, U any] struct {
		config     HandlerConfig
		handleFunc handleRequestDelegate[T, U]
	}

	handleRequestDelegate[T, U any] func(ctx context.Context, request T) (*Response[U], serr.Error)

	HandlerArgument interface {
		Source() ArgumentDataSource
	}

	ArmoryPrincipalArgument struct {
		*iam.ArmoryCloudPrincipal
	}

	ArgumentDataSource int
)

const (
	PathContextSource  ArgumentDataSource = 0
	QueryContextSource                    = 1
	AuthContextSource                     = 2
)

func (r *handler[REQUEST, RESPONSE]) Config() HandlerConfig {
	return r.config
}

func (r *handler[REQUEST, RESPONSE]) GetGinHandlerFn(log *zap.SugaredLogger, requestValidator *validator.Validate, config *handlerDTO) gin.HandlerFunc {
	return ginHOF(r.handleFunc, config, requestValidator, log)
}

func (ArmoryPrincipalArgument) Source() ArgumentDataSource {
	return AuthContextSource
}

// NewHandler creates a Handler from a handler function and server.HandlerConfig
func NewHandler[REQUEST, RESPONSE any](f func(ctx context.Context, request REQUEST) (*Response[RESPONSE], serr.Error), config HandlerConfig) Handler {
	return &handler[REQUEST, RESPONSE]{
		config:     config,
		handleFunc: f,
	}
}

func NewEnrichedHandler1[REQUEST, RESPONSE any, CTX HandlerArgument](f func(ctx context.Context, request REQUEST, arg1 CTX) (*Response[RESPONSE], serr.Error), config HandlerConfig) Handler {
	var delegate handleRequestDelegate[REQUEST, RESPONSE] = func(ctx context.Context, r REQUEST) (*Response[RESPONSE], serr.Error) {
		arg, err := extractHandlerArgumentFromContext[CTX](ctx)
		if nil != err {
			return nil, err
		}

		return f(ctx, r, *arg)
	}

	return &handler[REQUEST, RESPONSE]{
		config:     config,
		handleFunc: delegate,
	}
}

func NewEnrichedHandler2[REQUEST, RESPONSE any, CTX1 HandlerArgument, CTX2 HandlerArgument](f func(ctx context.Context, request REQUEST, arg1 CTX1, arg2 CTX2) (*Response[RESPONSE], serr.Error), config HandlerConfig) Handler {

	var delegate handleRequestDelegate[REQUEST, RESPONSE] = func(ctx context.Context, r REQUEST) (*Response[RESPONSE], serr.Error) {
		arg1, err := extractHandlerArgumentFromContext[CTX1](ctx)
		if nil != err {
			return nil, err
		}
		arg2, err := extractHandlerArgumentFromContext[CTX2](ctx)
		if nil != err {
			return nil, err
		}
		return f(ctx, r, *arg1, *arg2)
	}

	return &handler[REQUEST, RESPONSE]{
		config:     config,
		handleFunc: delegate,
	}
}

func NewEnrichedHandler3[REQUEST, RESPONSE any, CTX1 HandlerArgument, CTX2 HandlerArgument, CTX3 HandlerArgument](f func(ctx context.Context, request REQUEST, arg1 CTX1, arg2 CTX2, arg3 CTX3) (*Response[RESPONSE], serr.Error), config HandlerConfig) Handler {

	var delegate handleRequestDelegate[REQUEST, RESPONSE] = func(ctx context.Context, r REQUEST) (*Response[RESPONSE], serr.Error) {
		arg1, err := extractHandlerArgumentFromContext[CTX1](ctx)
		if nil != err {
			return nil, err
		}
		arg2, err := extractHandlerArgumentFromContext[CTX2](ctx)
		if nil != err {
			return nil, err
		}
		arg3, err := extractHandlerArgumentFromContext[CTX3](ctx)
		if nil != err {
			return nil, err
		}
		return f(ctx, r, *arg1, *arg2, *arg3)
	}

	return &handler[REQUEST, RESPONSE]{
		config:     config,
		handleFunc: delegate,
	}
}

func extractHandlerArgumentFromContext[CTX HandlerArgument](c context.Context) (*CTX, serr.Error) {
	var arg CTX
	switch arg.Source() {
	case PathContextSource:
		err := extract(c, extractPathDetails, &arg)
		return &arg, err

	case QueryContextSource:
		err := extract(c, extractQueryDetails, &arg)
		return &arg, err

	case AuthContextSource:
		principal, err := ExtractPrincipalFromContext(c)
		var retValue interface{} = &ArmoryPrincipalArgument{principal}
		return retValue.(*CTX), err
	}
	return nil, serr.NewSimpleError(fmt.Sprintf("not supported argument source %d", arg.Source()), nil)
}
