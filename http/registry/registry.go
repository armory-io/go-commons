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

package registry

import (
	"context"
	"encoding/json"
	"fmt"
	armoryhttp "github.com/armory-io/go-commons/http"
	"github.com/armory-io/go-commons/iam"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"io"
	"net/http"
	"reflect"
)

type (
	HandlerRegistry struct {
		log *zap.SugaredLogger
		gin *gin.Engine
	}

	Handler interface {
		Register(g gin.IRoutes, log *zap.SugaredLogger)
		Config() HandlerConfig // For testing.
	}

	HandlerConfig struct {
		Path       string
		Validators []Validator
		StatusCode int
		Method     string
	}

	Validator func(p *iam.ArmoryCloudPrincipal) (string, bool)

	requestHandler[REQUEST any] struct {
		config     HandlerConfig
		handleFunc func(ctx context.Context, request REQUEST) error
	}

	requestResponseHandler[REQUEST, RESPONSE any] struct {
		config     HandlerConfig
		handleFunc func(ctx context.Context, params REQUEST) (*RESPONSE, error)
	}
)

func NewRegistry(log *zap.SugaredLogger, g *gin.Engine) *HandlerRegistry {
	return &HandlerRegistry{
		log: log,
		gin: g,
	}
}

func (r *HandlerRegistry) RegisterHandlers(handlers ...Handler) {
	for _, handler := range handlers {
		r.log.Infof("Registering REST handler %s %s", handler.Config().Method, handler.Config().Path)
		handler.Register(r.gin, r.log)
	}
}

func NewRequestResponseHandler[REQUEST, RESPONSE any](f func(ctx context.Context, request REQUEST) (*RESPONSE, error), config HandlerConfig) Handler {
	return &requestResponseHandler[REQUEST, RESPONSE]{
		config:     config,
		handleFunc: f,
	}
}

func NewRequestHandler[REQUEST any](f func(ctx context.Context, request REQUEST) error, config HandlerConfig) Handler {
	return &requestHandler[REQUEST]{
		config:     config,
		handleFunc: f,
	}
}

func (r *requestResponseHandler[REQUEST, RESPONSE]) Register(g gin.IRoutes, log *zap.SugaredLogger) {
	registerHandler(g, log, r.config, func(c *gin.Context) error {
		response, err := r.delegate(c)
		if err != nil {
			return err
		}
		writeStatusCode(r.config, c.Writer)
		return json.NewEncoder(c.Writer).Encode(response)
	})
}

func writeStatusCode(config HandlerConfig, writer http.ResponseWriter) {
	statusCode := http.StatusOK
	if config.StatusCode != 0 {
		statusCode = config.StatusCode
	}

	writer.WriteHeader(statusCode)
}

func (r *requestResponseHandler[REQUEST, RESPONSE]) Config() HandlerConfig {
	return r.config
}

func (r *requestResponseHandler[REQUEST, RESPONSE]) delegate(c *gin.Context) (*RESPONSE, error) {
	method := r.config.Method
	if method == http.MethodGet || method == http.MethodDelete {
		var req REQUEST
		if err := decodeInto(extract(c), &req); err != nil {
			return nil, err
		}

		return r.handleFunc(c.Request.Context(), req)
	} else if method == http.MethodPost || method == http.MethodPut {
		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return nil, err
		}

		var req REQUEST
		if err := json.Unmarshal(b, &req); err != nil {
			return nil, err
		}

		return r.handleFunc(c.Request.Context(), req)
	}

	return nil, fmt.Errorf("cannot handle request method %q", method)
}

func (r *requestHandler[REQUEST]) Register(g gin.IRoutes, log *zap.SugaredLogger) {
	registerHandler(g, log, r.config, func(c *gin.Context) error {
		if err := r.delegate(c); err != nil {
			return err
		}
		writeStatusCode(r.config, c.Writer)
		return nil
	})
}

func (r *requestHandler[REQUEST]) Config() HandlerConfig {
	return r.config
}

func (r *requestHandler[REQUEST]) delegate(c *gin.Context) error {
	var params REQUEST
	if err := decodeInto(extract(c), &params); err != nil {
		return err
	}
	return r.handleFunc(c.Request.Context(), params)
}

func registerHandler(g gin.IRoutes, log *zap.SugaredLogger, config HandlerConfig, f func(c *gin.Context) error) {
	g.Handle(config.Method, config.Path, func(c *gin.Context) {
		if err := handle(c, config, f); err != nil {
			errorID := uuid.NewString()
			writeErr := writeError(c.Writer, err, errorID)
			fields := []any{
				"Error", multierr.Combine(err, writeErr).Error(),
				"Path", config.Path,
				"Method", config.Method,
				"ErrorID", errorID,
			}

			principal, err := iam.ExtractPrincipalFromContext(c.Request.Context())
			if err != nil {
				log.With(fields...).Errorf("Could not extract principal from request: %s", err)
			}
			if principal != nil {
				fields = append(fields, "Tenant", principal.Tenant())
			}
			log.With(fields).Error("Could not handle request")
		}
	})
}

func handle(c *gin.Context, config HandlerConfig, f func(c *gin.Context) error) error {
	c.Writer.Header().Add("Content-Type", "application/json")
	if err := validate(c.Request.Context(), config); err != nil {
		return err
	}

	return f(c)
}

func validate(ctx context.Context, config HandlerConfig) error {
	if len(config.Validators) > 0 {
		return validatePrincipal(ctx, config)
	} else {
		return armoryhttp.NewStatusError("not allowed", http.StatusForbidden)
	}
}

func validatePrincipal(ctx context.Context, config HandlerConfig) error {
	principal, err := iam.ExtractPrincipalFromContext(ctx)
	if err != nil {
		return err
	}

	for _, v := range config.Validators {
		if msg, ok := v(principal); !ok {
			return armoryhttp.NewStatusError(msg, http.StatusForbidden)
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

func statusCodeFromError(err error) int {
	sErr, ok := err.(*armoryhttp.StatusError)
	if ok {
		return sErr.StatusCode()
	}
	return http.StatusInternalServerError
}

func writeError(writer http.ResponseWriter, err error, errorID string) error {
	writer.WriteHeader(statusCodeFromError(err))
	return json.NewEncoder(writer).Encode(armoryhttp.BackstopError{
		ErrorID: errorID,
		Errors:  armoryhttp.Errors{{Message: err.Error()}},
	})
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
