package management

import (
	"context"
	"github.com/armory-io/go-commons/maputils"
	"github.com/armory-io/go-commons/server"
	"github.com/armory-io/go-commons/server/serr"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net/http"
)

type InfoController struct {
	log          *zap.SugaredLogger
	contributors []infoContributor
}

type InfoBuilder struct {
	content map[string]any
}

func (i *InfoBuilder) WithDetail(key string, value any) {
	i.content[key] = value
}

func (i *InfoBuilder) WithDetails(details map[string]any) {
	i.content = maputils.MergeSources(i.content, details)
}

type InfoResponse map[string]any

type infoContributor interface {
	Contribute(builder *InfoBuilder)
}

type infoContributors struct {
	fx.In
	InfoContributors []infoContributor `group:"info"`
}

type InfoContributor struct {
	fx.Out
	InfoContributor infoContributor `group:"info"`
}

func NewInfoController(log *zap.SugaredLogger, c infoContributors) server.ManagementController {
	return server.ManagementController{
		Controller: &InfoController{
			log:          log,
			contributors: c.InfoContributors,
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

func (i InfoController) infoHandler(_ context.Context, _ server.Void) (*server.Response[map[string]any], serr.Error) {
	ib := &InfoBuilder{
		content: make(map[string]any),
	}
	for _, c := range i.contributors {
		c.Contribute(ib)
	}
	return server.SimpleResponse(ib.content), nil
}
