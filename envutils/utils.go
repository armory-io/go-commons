/*
 * Copyright 2022 Armory, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package envutils

import (
	"os"
	"strings"
)

const (
	armoryApplicationName    = "ARMORY_APPLICATION_NAME"
	armoryEnvironmentName    = "ARMORY_ENVIRONMENT_NAME"
	armoryReplicaSetName     = "ARMORY_REPLICA_SET_NAME"
	armoryApplicationVersion = "ARMORY_APPLICATION_VERSION"
	armoryDeploymentId       = "ARMORY_DEPLOYMENT_ID"
	applicationName          = "APPLICATION_NAME"
	applicationEnv           = "APPLICATION_ENVIRONMENT"
	applicationVersion       = "APPLICATION_VERSION"
	LoggerType               = "LOGGER_TYPE"
	LoggerLevel              = "LOGGER_LEVEL"
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

// GetApplicationName returns the value of the ARMORY_APPLICATION_NAME env var else it defaults to empty string
func GetApplicationName() string {
	name := os.Getenv(applicationName)
	if name == "" {
		name = os.Getenv(armoryApplicationName)
	}
	return strings.ToLower(name)
}

// GetEnvironmentName returns the value of the ARMORY_ENVIRONMENT_NAME env var if present else it defaults to local
func GetEnvironmentName() string {
	envName := os.Getenv(applicationEnv)
	if envName == "" {
		envName = os.Getenv(armoryEnvironmentName)
	}
	if envName == "" {
		envName = local
	}
	return strings.ToLower(envName)
}

// GetReplicaSetName returns the value of the ARMORY_REPLICA_SET_NAME env var if present else an empty string
func GetReplicaSetName() string {
	return os.Getenv(armoryReplicaSetName)
}

// GetApplicationVersion returns the value of ARMORY_APPLICATION_VERSION or else defaults to "unset"
func GetApplicationVersion() string {
	version := os.Getenv(applicationVersion)
	if version == "" {
		version = os.Getenv(armoryApplicationVersion)
	}
	if version == "" {
		version = "unset"
	}
	return version
}

// GetApplicationLoggingType returns the logging type env var if set
func GetApplicationLoggingType() string {
	return os.Getenv(LoggerType)
}

// GetApplicationLoggingLevel returns the logging type env var if set
func GetApplicationLoggingLevel() string {
	return os.Getenv(LoggerLevel)
}

// GetDeploymentId Fetches the armory deployment id, if set
func GetDeploymentId() string {
	depId := os.Getenv(armoryDeploymentId)
	if depId == "" {
		depId = "unset"
	}
	return depId
}
