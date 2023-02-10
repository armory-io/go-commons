package server

import (
	"github.com/armory-io/go-commons/logging"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net/http"
	"testing"
)

const (
	test1JSON     = "application/test1+json"
	test1LinkJSON = "application/test1.link+json"
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
		}),
		NewHandler(noop, HandlerConfig{
			Path:       "/pipelines/kubernetes",
			Method:     http.MethodPost,
			StatusCode: http.StatusAccepted,
			Consumes:   test1LinkJSON,
			Produces:   test1JSON,
			Default:    false,
		}),
		NewHandler(noop, HandlerConfig{
			Path:       "/pipelines/kubernetes",
			Method:     http.MethodPost,
			StatusCode: http.StatusAccepted,
			Consumes:   test1JSON,
			Produces:   test1LinkJSON,
			Default:    false,
		}),
		NewHandler(noop, HandlerConfig{
			Path:       "/pipelines/kubernetes",
			Method:     http.MethodPost,
			StatusCode: http.StatusAccepted,
			Consumes:   test1LinkJSON,
			Produces:   test1LinkJSON,
			Default:    false,
		}),
	}
}

func (s *RegistryTestSuite) TestRegisterHandlersWithSameProducesAndConsumesCombination() {
	registryData := map[handlerDTOKey]map[handlerDTOMimeTypeKey]*handlerDTO{}
	for _, handler := range s.controller.Handlers() {
		err := configureHandler(handler, s.controller, s.log, nil, registryData)
		s.NoError(err, "all handlers should register")
	}

	err := configureHandler(s.controller.Handlers()[0], s.controller, s.log, nil, registryData)
	s.ErrorIs(err, ErrDuplicateHandlerRegistered)
}
