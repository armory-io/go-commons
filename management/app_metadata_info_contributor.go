package management

import (
	"github.com/armory-io/go-commons/management/info"
	"github.com/armory-io/go-commons/metadata"
)

func AppMetaInfoContributor(app metadata.ApplicationMetadata) info.InfoContributorOut {
	return info.InfoContributorOut{
		InfoContributor: &appMetaInfoContributor{
			app: app,
		},
	}
}

type appMetaInfoContributor struct {
	app metadata.ApplicationMetadata
}

func (a appMetaInfoContributor) Contribute(builder *info.InfoBuilder) {
	builder.WithDetail("application", a.app)
}
