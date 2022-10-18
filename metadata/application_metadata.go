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
	LoggingType string
}

func ApplicationMetadataProvider() ApplicationMetadata {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	return ApplicationMetadata{
		Name:        envutils.GetApplicationName(),
		Version:     envutils.GetApplicationVersion(),
		Environment: envutils.GetEnvironmentName(),
		Replicaset:  envutils.GetReplicaSetName(),
		LoggingType: envutils.GetApplicationLoggingType(),
		Hostname:    hostname,
	}
}

var Module = fx.Options(
	fx.Provide(ApplicationMetadataProvider),
)
