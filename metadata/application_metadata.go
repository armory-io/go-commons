package metadata

import (
	"github.com/armory-io/go-commons/envutils"
	"github.com/google/uuid"
	"go.uber.org/fx"
	"os"
)

type ApplicationMetadata struct {
	Name         string `json:"name,omitempty"`
	Version      string `json:"version,omitempty"`
	Environment  string `json:"environment,omitempty"`
	Replicaset   string `json:"replicaset,omitempty"`
	DeploymentId string `json:"deploymentId,omitempty"`
	Hostname     string `json:"hostname,omitempty"`
	InstanceId   string `json:"instanceId"`

	LoggingType  string `json:"-"`
	LoggingLevel string `json:"-"`
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
		DeploymentId: envutils.GetDeploymentId(),
		Hostname:     hostname,
		InstanceId:   uuid.NewString(),

		LoggingType:  envutils.GetApplicationLoggingType(),
		LoggingLevel: envutils.GetApplicationLoggingLevel(),
	}
}

var Module = fx.Options(
	fx.Provide(ApplicationMetadataProvider),
)
