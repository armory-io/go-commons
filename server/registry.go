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
	"errors"
	"fmt"
	"github.com/armory-io/go-commons/iam"
	"github.com/armory-io/go-commons/management/info"
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

var ErrDuplicateHandlerRegistered = errors.New("there was a duplicate handler registered")

type (
	handlerDTOKey struct {
		path   string
		method string
	}

	handlerDTOMimeTypeKey struct {
		consumes string
		produces string
	}

	handlerDTO struct {
		Path               string                `json:"-"`
		Method             string                `json:"method"`
		AuthZValidators    []AuthZValidatorV2Fn  `json:"-"`
		AuthOptOut         bool                  `json:"authOptOut"`
		Consumes           string                `json:"consumes"`
		Produces           string                `json:"produces"`
		StatusCode         int                   `json:"statusCode"`
		HandlerFn          gin.HandlerFunc       `json:"-"`
		MediaType          contenttype.MediaType `json:"-"`
		ConsumesMediaType  contenttype.MediaType `json:"-"`
		Default            bool                  `json:"default"`
		ResponseProcessors []ResponseProcessorFn `json:"-"`
	}
)

type handlerRegistry struct {
	name   string
	logger *zap.SugaredLogger
	data   map[handlerDTOKey]map[handlerDTOMimeTypeKey]*handlerDTO
}

type registerHandlersInput struct {
	AuthRequiredGroup    *gin.RouterGroup
	AuthNotEnforcedGroup *gin.RouterGroup
}

type iHandlerRegistry interface {
	registerHandlers(in registerHandlersInput) error
	Contribute(builder *info.InfoBuilder)
}

// Contribute implements the management.infoContributor interface so we can add available routes at the /info endpoint
func (r *handlerRegistry) Contribute(builder *info.InfoBuilder) {
	data := make(map[string][]*handlerDTO)
	for k, v := range r.data {
		data[k.path] = maps.Values(v)
	}
	builder.WithDetails(map[string]any{
		"routes": map[string]any{
			r.name: data,
		},
	})
}

func (r *handlerRegistry) registerHandlers(in registerHandlersInput) error {
	for key, handlersByMimeType := range r.data {
		authOptOut := maps.Values(handlersByMimeType)[0].AuthOptOut

		// Ensure that all in handlers for the multi-mime type handler have the same auth settings
		matches := lo.PickBy(handlersByMimeType, func(mimeTypeKey handlerDTOMimeTypeKey, handler *handlerDTO) bool {
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

func createMultiMimeTypeFn(handlersByMimeType map[handlerDTOMimeTypeKey]*handlerDTO, logger *zap.SugaredLogger) gin.HandlerFunc {
	values := maps.Values(handlersByMimeType)
	// sort available in reverse lexicographical order, so that the newest version is chosen by default when no accept header is present
	sort.Slice(values, func(i, j int) bool { return values[i].Produces > values[j].Produces })
	// if a handler is listed as default bubble that to the top of the list so that is chosen by default when no accept header is present
	sort.Slice(values, func(i, j int) bool { return values[i].Default })
	available := lo.Map(values, func(hDTO *handlerDTO, _ int) contenttype.MediaType {
		return hDTO.MediaType
	})
	availableConsumes := lo.Map(values, func(hDTO *handlerDTO, _ int) contenttype.MediaType {
		return hDTO.ConsumesMediaType
	})

	return func(c *gin.Context) {
		accept := c.Request.Header.Get("Accept")
		if accept == "" {
			accept = "*/*"
		}
		contentType := c.Request.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "*/*"
		}
		availableCombinations := lo.Map(values, func(hDTO *handlerDTO, _ int) handlerDTOMimeTypeKey {
			return handlerDTOMimeTypeKey{hDTO.Consumes, hDTO.Produces}
		})
		// TODO add params to context
		amt, _, err := contenttype.GetAcceptableMediaTypeFromHeader(accept, available)
		if err != nil {
			handleContentTypesMismatch(c, availableCombinations, c.ContentType(), accept, err, logger)
			return
		}
		// for backward compatibility, we should accept super type of Accept header as a valid Content-Type
		availableConsumes = append(availableConsumes, getMediaSuperType(amt))
		cmt, _, err := contenttype.GetAcceptableMediaTypeFromHeader(contentType, availableConsumes)
		if err != nil {
			handleContentTypesMismatch(c, availableCombinations, c.ContentType(), accept, err, logger)
			return
		}
		// execute the handler func for the requested MIME type
		handler := handlersByMimeType[handlerDTOMimeTypeKey{
			consumes: cmt.MIME(),
			produces: amt.MIME(),
		}]

		if handler == nil {
			//If there was no consume/produces match, default to the first matching producer
			handler = findAcceptableDefaultHandler(values, amt, cmt)

			if handler == nil {
				handleContentTypesMismatch(c, availableCombinations, c.ContentType(), accept, err, logger)
				return
			}

		}
		handler.HandlerFn(c)
	}
}

//findAcceptableDefaultHandler An acceptable match will be a matching produces MediaType and one that has the same consumes Type and a subset of the Subtype
func findAcceptableDefaultHandler(handlers []*handlerDTO, produces contenttype.MediaType, consumes contenttype.MediaType) *handlerDTO {
	for _, dto := range handlers {
		//if a handler matches on `produces`
		if dto.MediaType.MIME() == produces.MIME() {
			//and consumes is any, return first result
			if consumes.MIME() == "*/*" {
				return dto
			}
			//and produces Type matches and has a simplified Subtype match
			// i.e. `application/json` is provided for consumes, handler has consumes `application/my.specific+json`, it will match
			if dto.ConsumesMediaType.Type == produces.Type && strings.HasSuffix(dto.ConsumesMediaType.Subtype, "+"+consumes.Subtype) {
				return dto
			}
		}
	}
	return nil
}

func getMediaSuperType(mediaType contenttype.MediaType) contenttype.MediaType {
	if !strings.Contains(mediaType.Subtype, "+") {
		return mediaType
	}
	split := strings.Split(mediaType.Subtype, "+")
	if len(split) != 2 {
		return mediaType
	}
	superType, err := contenttype.ParseMediaType(mediaType.Type + "/" + split[1])
	if err != nil {
		return mediaType
	}
	return superType
}

func handleContentTypesMismatch(c *gin.Context, availableCombinations []handlerDTOMimeTypeKey, contentType string, accept string, err error, logger *zap.SugaredLogger) {
	availableMimeTypes := lo.Map(availableCombinations, func(m handlerDTOMimeTypeKey, _ int) string {
		return fmt.Sprintf("[Content-Type: %s, Accept: %s]", m.consumes, m.produces)
	})
	writeAndLogApiErrorThenAbort(c, serr.NewErrorResponseFromApiError(serr.APIError{
		Message: "Server can not produce requested content type for received content type",
		Metadata: map[string]any{
			"provided":  contentType,
			"requested": accept,
			"available": strings.Join(availableMimeTypes, ", "),
		},
		HttpStatusCode: http.StatusBadRequest,
	},
		serr.WithCause(err),
		serr.WithExtraDetailsForLogging(
			serr.KVPair{
				Key:   "provided-type",
				Value: contentType,
			},
			serr.KVPair{
				Key:   "requested-type",
				Value: accept,
			},
			serr.KVPair{
				Key:   "available-type-combinations",
				Value: strings.Join(availableMimeTypes, ", "),
			},
		)), logger)
}

func newHandlerRegistry(name string, logger *zap.SugaredLogger, requestValidator *validator.Validate, controllerCollections ...[]IController) (iHandlerRegistry, error) {
	registryData := make(map[handlerDTOKey]map[handlerDTOMimeTypeKey]*handlerDTO)
	for _, collection := range controllerCollections {
		for _, c := range collection {
			for _, h := range c.Handlers() {
				if err := configureHandler(h, c, logger, requestValidator, registryData); err != nil {
					return nil, err
				}
			}
		}
	}

	return &handlerRegistry{
		name:   name,
		logger: logger,
		data:   registryData,
	}, nil
}

func configureHandler(handler Handler, controller IController, logger *zap.SugaredLogger, requestValidator *validator.Validate, registryData map[handlerDTOKey]map[handlerDTOMimeTypeKey]*handlerDTO) error {
	validators := make([]AuthZValidatorV2Fn, 0)
	hDTO := &handlerDTO{
		Path:       strings.TrimSuffix(strings.TrimSpace(handler.Config().Path), "/"),
		Method:     strings.TrimSpace(handler.Config().Method),
		AuthOptOut: handler.Config().AuthOptOut,
		StatusCode: handler.Config().StatusCode,
		Default:    handler.Config().Default,
	}

	if handler.Config().AuthZValidator != nil {
		simpleHandler := func(c context.Context, p *iam.ArmoryCloudPrincipal) (string, bool) {
			return handler.Config().AuthZValidator(p)
		}
		validators = append(validators, simpleHandler)
	}
	if handler.Config().AuthZValidatorExtended != nil {
		validators = append(validators, handler.Config().AuthZValidatorExtended)
	}

	// Configure the Path with the controller provided prefix if present
	if c, ok := controller.(IControllerPrefix); ok {
		if c.Prefix() != "" {
			tPath := strings.TrimPrefix(strings.TrimSpace(handler.Config().Path), "/")
			np := strings.TrimSuffix(fmt.Sprintf("%s/%s", c.Prefix(), tPath), "/")
			hDTO.Path = np
		}
	}

	// Prepend the controller validator if defined, so that the controller validator is called first.
	if c, ok := controller.(IControllerAuthZValidator); ok {
		simpleHandler := func(ctx context.Context, p *iam.ArmoryCloudPrincipal) (string, bool) {
			return c.AuthZValidator(p)
		}
		validators = append(validators, simpleHandler)
	}
	if c, ok := controller.(IControllerAuthZValidatorV2); ok {
		validators = append(validators, c.AuthZValidator)
	}

	var iResponseProcessors []ResponseProcessorWithOrder
	if c, ok := controller.(IControllerPostResponseProcessor); ok {
		iResponseProcessors = c.ResponseProcessors()
	}
	sort.Slice(iResponseProcessors, func(i, j int) bool {
		return iResponseProcessors[i].Order < iResponseProcessors[j].Order
	})

	processors := lo.Map(iResponseProcessors, func(processor ResponseProcessorWithOrder, _ int) ResponseProcessorFn {
		return processor.Processor
	})

	perHandlerProcessors := handler.Config().responseProcessors
	if len(perHandlerProcessors) > 0 {
		processors = append(processors, perHandlerProcessors...)
	}

	hDTO.ResponseProcessors = processors

	if handler.Config().Produces != "" {
		hDTO.Produces = handler.Config().Produces
	} else {
		hDTO.Produces = "application/json"
	}

	if handler.Config().Consumes != "" {
		hDTO.Consumes = handler.Config().Consumes
	} else {
		hDTO.Consumes = "application/json"
	}

	mt, err := contenttype.ParseMediaType(hDTO.Produces)
	if err != nil {
		return multierr.Append(
			fmt.Errorf("failed to process mime type (%s) for handler with method: %s, path: %s", hDTO.Produces, hDTO.Method, hDTO.Path),
			err,
		)
	}

	hDTO.MediaType = mt
	cmt, err := contenttype.ParseMediaType(hDTO.Consumes)
	if err != nil {
		return multierr.Append(
			fmt.Errorf("failed to process mime type (%s) for handler with method: %s, path: %s", hDTO.Consumes, hDTO.Method, hDTO.Path),
			err,
		)
	}
	hDTO.ConsumesMediaType = cmt

	if hDTO.StatusCode == 0 {
		hDTO.StatusCode = http.StatusOK
	}

	hDTO.AuthZValidators = validators

	hDTO.HandlerFn = handler.GetGinHandlerFn(logger, requestValidator, hDTO)

	return registerHandler(hDTO, registryData)
}

func registerHandler(hDTO *handlerDTO, registryData map[handlerDTOKey]map[handlerDTOMimeTypeKey]*handlerDTO) error {
	key := handlerDTOKey{
		path:   hDTO.Path,
		method: hDTO.Method,
	}

	mimeTypeKey := handlerDTOMimeTypeKey{
		consumes: hDTO.Consumes,
		produces: hDTO.Produces,
	}

	if registryData[key] == nil {
		registryData[key] = make(map[handlerDTOMimeTypeKey]*handlerDTO)
	}

	if registryData[key][mimeTypeKey] != nil {
		return fmt.Errorf("failed to register handler for [Path: %s, Method: %s, Consumes: %s, Produces: %s] %w", hDTO.Path, hDTO.Method, hDTO.Consumes, hDTO.Produces, ErrDuplicateHandlerRegistered)
	}

	registryData[key][mimeTypeKey] = hDTO
	return nil
}
