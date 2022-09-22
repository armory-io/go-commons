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

package auth0

import (
    "encoding/json"
    "fmt"
    "github.com/armory-io/go-commons/secrets"
    "strings"
)

const (
    secretNamePrefix = "auth0-secrets-"
    region           = "us-west-2"

    EnvironmentProd    = "prod"
    EnvironmentStaging = "staging"
    EnvironmentDev     = "dev"
)

var (
    environments = []string{EnvironmentProd, EnvironmentStaging, EnvironmentDev}
)

type EnvironmentSecrets struct {
    ManagementClientID     string `json:"managementClientId"`
    ManagementClientSecret string `json:"managementClientSecret"`
    ManagementDomain       string `json:"managementDomain"`
    ClientID               string `json:"clientId"`
    ClientSecret           string `json:"clientSecret"`
    Domain                 string `json:"domain"`
}

func oneOf(s string, matches ...string) bool {
    for _, match := range matches {
        if match == s {
            return true
        }
    }
    return false
}

func GetEnvironmentSecrets(env string) (*EnvironmentSecrets, error) {
    if !oneOf(env, environments...) {
        return nil, fmt.Errorf("%w: must be one of (%s)", ErrUnknownEnvironment, strings.Join(environments, ", "))
    }

    client, err := secrets.NewAwsSecretsManagerClient(region)
    if err != nil {
        return nil, err
    }

    output, err := client.FetchSecret(secretNamePrefix + env)
    if err != nil {
        return nil, err
    }

    if output.SecretString == nil {
        return nil, fmt.Errorf("%w: %s%s", ErrSecretNotFound, secretNamePrefix, env)
    }

    var pair EnvironmentSecrets
    if err := json.Unmarshal([]byte(*output.SecretString), &pair); err != nil {
        return nil, err
    }

    return &pair, nil
}
