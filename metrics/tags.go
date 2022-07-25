package metrics

import (
	"os"
)

func getBaseTags(settings Settings) map[string]string {
	var environment string
	if len(settings.Environment) > 0 {
		environment = settings.Environment
	} else {
		environment = "UNKNOWN"
	}

	return map[string]string{
		"appName":     settings.ApplicationName,
		"version":     settings.Version,
		"hostname":    getOptionalEnvVar("HOSTNAME"),
		"environment": environment,
		"replicaset":  getOptionalEnvVar("ARMORY_REPLICA_SET_NAME"),
	}
}

func getOptionalEnvVar(key string) string {
	var value string
	if len(os.Getenv(key)) > 0 {
		value = os.Getenv(key)
	} else {
		value = "UNKNOWN"
	}
	return value
}
