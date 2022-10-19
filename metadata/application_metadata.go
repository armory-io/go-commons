package metadata

import (
	"github.com/armory-io/go-commons/envutils"
	"go.uber.org/fx"
	"os"
)

type ApplicationMetadata struct {
	Name         string `json:"name,omitempty"`
	Version      string `json:"version,omitempty"`
	Environment  string `json:"environment,omitempty"`
	Replicaset   string `json:"replicaset,omitempty"`
	Hostname     string `json:"hostname,omitempty"`
	LoggingType  string `json:"-"`
	DeploymentId string `json:"deploymentId,omitempty"`
}

func ApplicationMetadataProvider() ApplicationMetadata {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	return ApplicationMetadata{
		Name:         envutils.GetApplicationName(),
		Version:      envutils.GetApplicationVersion(),
		Environment:  envutils.GetEnvironmentName(),
		Replicaset:   envutils.GetReplicaSetName(),
		LoggingType:  envutils.GetApplicationLoggingType(),
		DeploymentId: envutils.GetDeploymentId(),
		Hostname:     hostname,
	}
}

var Module = fx.Options(
	fx.Provide(ApplicationMetadataProvider),
)
