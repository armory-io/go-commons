package management

import (
	"github.com/armory-io/go-commons/metadata"
)

func AppMetaInfoContributor(app metadata.ApplicationMetadata) InfoContributor {
	return InfoContributor{
		InfoContributor: &appMetaInfoContributor{
			app: app,
		},
	}
}

type appMetaInfoContributor struct {
	app metadata.ApplicationMetadata
}

func (a appMetaInfoContributor) Contribute(builder *InfoBuilder) {
	builder.WithDetail("application", a.app)
}
