package info

import (
	"github.com/armory-io/go-commons/maputils"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type infoContributors struct {
	fx.In
	InfoContributors []InfoContributor `group:"info"`
}

type InfoContributor interface {
	Contribute(builder *InfoBuilder)
}

type InfoContributorOut struct {
	fx.Out
	InfoContributor InfoContributor `group:"info"`
}

type InfoContributorsOut struct {
	fx.Out
	InfoContributors []InfoContributor `group:"info"`
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

type InfoService struct {
	log          *zap.SugaredLogger
	contributors []InfoContributor
}

func New(log *zap.SugaredLogger, c infoContributors) *InfoService {
	return &InfoService{
		log:          log,
		contributors: c.InfoContributors,
	}
}

// AddInfoContributor a method to register an info contributor post DI lifecycle phase
func (is *InfoService) AddInfoContributor(contributor InfoContributor) {
	is.contributors = append(is.contributors, contributor)
}

func (is *InfoService) GetInfoContent() *map[string]any {
	ib := &InfoBuilder{
		content: make(map[string]any),
	}
	for _, c := range is.contributors {
		c.Contribute(ib)
	}
	return &ib.content
}
