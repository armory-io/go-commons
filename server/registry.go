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
	"fmt"
	"github.com/armory-io/go-commons/server/serr"
	"github.com/elnormous/contenttype"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"net/http"
	"sort"
	"strings"
)

type (
	handlerDTOKey struct {
		path   string
		method string
	}

	handlerDTO struct {
		Path            string
		Method          string
		AuthZValidators []AuthZValidatorFn
		AuthOptOut      bool
		Consumes        string
		Produces        string
		StatusCode      int
		HandlerFn       gin.HandlerFunc
		MediaType       contenttype.MediaType
		Default         bool
	}
)

type handlerRegistry struct {
	logger *zap.SugaredLogger
	data   map[handlerDTOKey]map[string]*handlerDTO
}

type registerHandlersInput struct {
	AuthRequiredGroup    *gin.RouterGroup
	AuthNotEnforcedGroup *gin.RouterGroup
}

type iHandlerRegistry interface {
	registerHandlers(in registerHandlersInput) error
}

func (r *handlerRegistry) registerHandlers(in registerHandlersInput) error {
	for key, handlersByMimeType := range r.data {
		authOptOut := maps.Values(handlersByMimeType)[0].AuthOptOut

		// Ensure that all in handlers for the multi-mime type handler have the same auth settings
		matches := lo.PickBy(handlersByMimeType, func(contentType string, handler *handlerDTO) bool {
			return handler.AuthOptOut != authOptOut
		})
		if len(matches) > 0 {
			return fmt.Errorf("can not register composite multi-mime type handler with for method: %s and path: %s because all handers do not have the same AuthOptOut flag configured", key.method, key.path)
		}

		// Ensure that only 1 handler for the multi-mime type handler is marked as default
		defaultCount := lo.Filter(maps.Values(handlersByMimeType), func(handler *handlerDTO, _ int) bool {
			return handler.Default
		})
		if len(defaultCount) > 1 {
			return fmt.Errorf("can not register composite multi-mime type handler with for method: %s and path: %s because more than 1 hander was marked as the default", key.method, key.path)
		}

		fn := createMultiMimeTypeFn(handlersByMimeType, r.logger)

		if authOptOut {
			in.AuthNotEnforcedGroup.Handle(key.method, key.path, fn)
		} else {
			in.AuthRequiredGroup.Handle(key.method, key.path, fn)
		}
	}

	return nil
}

func createMultiMimeTypeFn(handlersByMimeType map[string]*handlerDTO, logger *zap.SugaredLogger) gin.HandlerFunc {
	values := maps.Values(handlersByMimeType)
	// sort available in reverse lexicographical order, so that the newest version is chosen by default when no accept header is present
	sort.Slice(values, func(i, j int) bool { return values[i].Produces > values[j].Produces })
	// if a handler is listed as default bubble that to the top of the list so that is chosen by default when no accept header is present
	sort.Slice(values, func(i, j int) bool { return values[i].Default })
	available := lo.Map(values, func(hDTO *handlerDTO, _ int) contenttype.MediaType {
		return hDTO.MediaType
	})
	return func(c *gin.Context) {
		accept := c.Request.Header.Get("Accept")
		if accept == "" {
			accept = "*/*"
		}

		// TODO add params to context
		mt, _, err := contenttype.GetAcceptableMediaTypeFromHeader(accept, available)
		if err != nil {
			availableMimeTypes := lo.Map(available, func(m contenttype.MediaType, _ int) string {
				return m.String()
			})
			writeAndLogApiErrorThenAbort(c, serr.NewErrorResponseFromApiError(serr.APIError{
				Message: "Server can not produce requested content type",
				Metadata: map[string]any{
					"requested": accept,
					"available": strings.Join(availableMimeTypes, ", "),
				},
				HttpStatusCode: http.StatusBadRequest,
			},
				serr.WithCause(err),
				serr.WithExtraDetailsForLogging(
					serr.KVPair{
						Key:   "requested-type",
						Value: accept,
					},
					serr.KVPair{
						Key:   "available-types",
						Value: strings.Join(availableMimeTypes, ", "),
					},
				)), logger)
			return
		}

		// execute the handler func for the requested MIME type
		handlersByMimeType[mt.MIME()].HandlerFn(c)
	}
}

func newHandlerRegistry(
	logger *zap.SugaredLogger,
	requestValidator *validator.Validate,
	controllerCollections ...[]IController,
) (iHandlerRegistry, error) {
	registryData := make(map[handlerDTOKey]map[string]*handlerDTO)
	for _, collection := range controllerCollections {
		for _, c := range collection {
			for _, h := range c.Handlers() {
				var validators []AuthZValidatorFn
				hDTO := &handlerDTO{
					Path:            strings.TrimSuffix(strings.TrimSpace(h.Config().Path), "/"),
					Method:          strings.TrimSpace(h.Config().Method),
					AuthZValidators: validators,
					AuthOptOut:      h.Config().AuthOptOut,
					StatusCode:      h.Config().StatusCode,
					Default:         h.Config().Default,
				}

				if h.Config().AuthZValidator != nil {
					validators = append(validators, h.Config().AuthZValidator)
				}

				// Configure the Path with the controller provided prefix if present
				if c, ok := c.(IControllerPrefix); ok {
					if c.Prefix() != "" {
						tPath := strings.TrimPrefix(strings.TrimSpace(h.Config().Path), "/")
						np := strings.TrimSuffix(fmt.Sprintf("%s/%s", c.Prefix(), tPath), "/")
						hDTO.Path = np
					}
				}

				// Prepend the controller validator if defined, so that the controller validator is called first.
				if c, ok := c.(IControllerAuthZValidator); ok {
					validators = append([]AuthZValidatorFn{c.AuthZValidator}, validators...)
				}

				if h.Config().Produces != "" {
					hDTO.Produces = h.Config().Produces
				} else {
					hDTO.Produces = "application/json"
				}

				if h.Config().Consumes != "" {
					hDTO.Consumes = h.Config().Consumes
				} else {
					hDTO.Consumes = "application/json"
				}

				mt, err := contenttype.ParseMediaType(hDTO.Produces)
				if err != nil {
					return nil, multierr.Append(
						fmt.Errorf("failed to process mime type (%s) for handler with method: %s, path: %s", hDTO.Produces, hDTO.Method, hDTO.Path),
						err,
					)
				}
				hDTO.MediaType = mt

				if hDTO.StatusCode == 0 {
					hDTO.StatusCode = http.StatusOK
				}

				key := handlerDTOKey{
					path:   hDTO.Path,
					method: hDTO.Method,
				}

				if registryData[key] == nil {
					registryData[key] = make(map[string]*handlerDTO)
				}

				if registryData[key][hDTO.Produces] != nil {
					return nil, fmt.Errorf("failed to register hander for [Path: %s, Method: %s, Produces: %s] there was already a registered handler", hDTO.Path, hDTO.Method, hDTO.Produces)
				}

				hDTO.HandlerFn = h.GetGinHandlerFn(logger, requestValidator, hDTO)
				registryData[key][hDTO.Produces] = hDTO
			}
		}
	}

	return &handlerRegistry{
		logger: logger,
		data:   registryData,
	}, nil
}
