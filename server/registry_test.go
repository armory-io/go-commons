package server

import (
	"github.com/armory-io/go-commons/logging"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

const (
	test1JSON       = "application/test1+json"
	test1LinkJSON   = "application/test1.link+json"
	applicationJSON = "application/json"
)

type RegistryTestSuite struct {
	log *zap.SugaredLogger
	suite.Suite
	controller testController
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryTestSuite))
}

func (s *RegistryTestSuite) SetupSuite() {
	logger, _ := logging.StdArmoryDevLogger(zapcore.InfoLevel)
	s.log = logger.Sugar()
	s.controller = testController{s.log}
}

type testController struct {
	log *zap.SugaredLogger
}

func (p testController) Handlers() []Handler {
	return []Handler{
		NewHandler(noop, HandlerConfig{
			Path:       "/pipelines/kubernetes",
			Method:     http.MethodPost,
			StatusCode: http.StatusAccepted,
			Consumes:   test1JSON,
			Produces:   test1JSON,
			Default:    true,
			AuthOptOut: true,
		}),
		NewHandler(noop, HandlerConfig{
			Path:       "/pipelines/kubernetes",
			Method:     http.MethodPost,
			StatusCode: http.StatusAccepted,
			Consumes:   test1LinkJSON,
			Produces:   test1JSON,
			Default:    false,
			AuthOptOut: true,
		}),
		NewHandler(noop, HandlerConfig{
			Path:       "/pipelines/kubernetes",
			Method:     http.MethodPost,
			StatusCode: http.StatusAccepted,
			Consumes:   test1JSON,
			Produces:   test1LinkJSON,
			Default:    false,
			AuthOptOut: true,
		}),
		NewHandler(noop, HandlerConfig{
			Path:       "/pipelines/kubernetes",
			Method:     http.MethodPost,
			StatusCode: http.StatusAccepted,
			Consumes:   test1LinkJSON,
			Produces:   test1LinkJSON,
			Default:    false,
			AuthOptOut: true,
		}),
	}
}

func (s *RegistryTestSuite) TestRegisterHandlersWithSameProducesAndConsumesCombination() {
	// When handlers are registered, there is no issue
	registryData := map[handlerDTOKey]map[handlerDTOMimeTypeKey]*handlerDTO{}
	for _, handler := range s.controller.Handlers() {
		err := configureHandler(handler, s.controller, s.log, nil, registryData)
		s.NoError(err, "all handlers should register")
	}

	// When a duplicate handler is registered, we get an error
	err := configureHandler(s.controller.Handlers()[0], s.controller, s.log, nil, registryData)
	s.ErrorIs(err, ErrDuplicateHandlerRegistered)

	// We can use the registered handler even when a super type (i.e. application/json is specified and there isn't a specific consumer for it)
	multiHandlerFn := createMultiMimeTypeFn(registryData[handlerDTOKey{
		path:   "/pipelines/kubernetes",
		method: http.MethodPost,
	}], s.log)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = &http.Request{
		URL: &url.URL{
			Path: "/pipelines/kubernetes",
		},
		Header: map[string][]string{
			// Request with a super type - should find a default subtype handler and execute
			"Content-Type": {applicationJSON},
			"Accept":       {test1LinkJSON},
		},
		Method: "POST",
	}
	multiHandlerFn(c)
}
