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

// Package auth0 is a toolkit for interacting with Auth0 APIs.
package auth0

import (
    "github.com/auth0/go-auth0/management"
)

func NewManagementClient(env string) (*management.Management, error) {
    config, err := GetEnvironmentSecrets(env)
    if err != nil {
        return nil, err
    }

    return management.New(config.ManagementDomain, management.WithClientCredentials(config.ManagementClientID, config.ManagementClientSecret))
}
