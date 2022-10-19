package management

import (
	"context"
	"github.com/armory-io/go-commons/management/info"
	"github.com/armory-io/go-commons/server"
	"github.com/armory-io/go-commons/server/serr"
	"go.uber.org/zap"
	"net/http"
)

type InfoController struct {
	log *zap.SugaredLogger
	is  *info.InfoService
}

type InfoResponse map[string]any

func NewInfoController(log *zap.SugaredLogger, is *info.InfoService) server.ManagementController {
	return server.ManagementController{
		Controller: &InfoController{
			log: log,
			is:  is,
		},
	}
}

func (i InfoController) Handlers() []server.Handler {
	return []server.Handler{
		server.NewHandler(i.infoHandler, server.HandlerConfig{
			Path:       "info",
			Method:     http.MethodGet,
			AuthOptOut: true,
		}),
	}
}

func (i InfoController) infoHandler(_ context.Context, _ server.Void) (*server.Response[*map[string]any], serr.Error) {
	return server.SimpleResponse(i.is.GetInfoContent()), nil
}
