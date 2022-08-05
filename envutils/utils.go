package envutils

import "os"

const (
	armoryApplicationName    = "ARMORY_APPLICATION_NAME"
	armoryEnvironmentName    = "ARMORY_ENVIRONMENT_NAME"
	armoryReplicaSetName     = "ARMORY_REPLICA_SET_NAME"
	armoryApplicationVersion = "ARMORY_APPLICATION_VERSION"
	local                    = "local"
)

// GetEnvVarOrDefault looks up an env var by its key and returns the value it's non-empty else the default is returned.
func GetEnvVarOrDefault(key, defaultValue string) string {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	return v
}

// GetArmoryApplicationName returns the value of the ARMORY_APPLICATION_NAME env var else it defaults to empty string
func GetArmoryApplicationName() string {
	return os.Getenv(armoryApplicationName)
}

// GetArmoryEnvironmentName returns the value of the ARMORY_ENVIRONMENT_NAME env var if present else it defaults to local
func GetArmoryEnvironmentName() string {
	return GetEnvVarOrDefault(armoryEnvironmentName, local)
}

// GetArmoryReplicaSetName returns the value of the ARMORY_REPLICA_SET_NAME env var if present else an empty string
func GetArmoryReplicaSetName() string {
	return os.Getenv(armoryReplicaSetName)
}

// GetArmoryApplicationVersion returns the value of ARMORY_APPLICATION_VERSION or else defaults to "unset"
func GetArmoryApplicationVersion() string {
	return GetEnvVarOrDefault(armoryApplicationVersion, "unset")
}
