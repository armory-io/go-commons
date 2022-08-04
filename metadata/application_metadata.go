package metadata

import (
	"github.com/armory-io/go-commons/envutils"
	"go.uber.org/fx"
	"os"
)

type ApplicationMetadata struct {
	Name        string
	Version     string
	Environment string
	Replicaset  string
	Hostname    string
}

func ApplicationMetadataProvider() ApplicationMetadata {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	return ApplicationMetadata{
		Name:        envutils.GetArmoryApplicationName(),
		Version:     envutils.GetArmoryApplicationVersion(),
		Environment: envutils.GetArmoryEnvironmentName(),
		Replicaset:  envutils.GetArmoryReplicaSetName(),
		Hostname:    hostname,
	}
}

var Module = fx.Options(
	fx.Provide(ApplicationMetadataProvider),
)
